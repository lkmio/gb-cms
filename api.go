package main

import (
	"context"
	"encoding/json"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	"time"
)

type ApiServer struct {
	router *mux.Router
}

var apiServer *ApiServer

func init() {
	apiServer = &ApiServer{
		router: mux.NewRouter(),
	}
}

type eventInfo struct {
	Stream     string `json:"stream"`      //Stream id
	Protocol   string `json:"protocol"`    //推拉流协议
	RemoteAddr string `json:"remote_addr"` //peer地址
}

func httpResponse2(w http.ResponseWriter, payload interface{}) {
	body, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT")
	w.Write(body)
}

func withCheckParams(f func(streamId, protocol string, w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		info := eventInfo{}
		err := HttpDecodeJSONBody(w, req, &info)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		f(info.Stream, info.Protocol, w, req)
	}
}

func startApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", withCheckParams(apiServer.OnPlay))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", withCheckParams(apiServer.OnPlayDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", withCheckParams(apiServer.OnPublish))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", withCheckParams(apiServer.OnPublishDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", withCheckParams(apiServer.OnIdleTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", withCheckParams(apiServer.OnReceiveTimeout))
	apiServer.router.HandleFunc("/api/v1/device/list", apiServer.OnDeviceList)
	http.Handle("/", apiServer.router)

	srv := &http.Server{
		Handler: apiServer.router,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	err := srv.ListenAndServe()

	if err != nil {
		panic(err)
	}
}

func (api *ApiServer) OnPlay(streamId, protocol string, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("play. protocol:%s stream id:%s", protocol, streamId)

	stream := StreamManager.Find(streamId)
	if stream != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	//发送invite
	split := strings.Split(streamId, "/")
	if len(split) != 2 {
		w.WriteHeader(http.StatusOK)
		return
	}

	deviceId := split[0]  //deviceId
	channelId := split[1] //channelId
	device := DeviceManager.Find(deviceId)

	if len(deviceId) != 20 || len(channelId) != 20 {
		w.WriteHeader(http.StatusOK)
		return
	}

	if device == nil {
		Sugar.Warnf("设备离线 id:%s", deviceId)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	stream = &Stream{Id: streamId, Protocol: "28181", ByeRequest: nil}
	if err := StreamManager.Add(stream); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var inviteOk bool
	defer func() {
		if !inviteOk {
			api.CloseStream(streamId)
			go CloseGBSource(streamId)
		}
	}()

	ssrc := GetLiveSSRC()
	ip, port, err := CreateGBSource(streamId, "UDP", "recvonly", ssrc)
	if err != nil {
		Sugar.Errorf("创建GBSource失败 err:%s", err.Error())
		return
	}

	inviteRequest, err := device.DoLive(channelId, ip, port, "RTP/AVP", "recvonly", ssrc)
	if err != nil {
		return
	}

	var bye sip.Request
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	SipUA.SendRequestWithContext(reqCtx, inviteRequest, gosip.WithResponseHandler(func(res sip.Response, request sip.Request) {
		if res.StatusCode() < 200 {

		} else if res.StatusCode() == 200 {
			ackRequest := sip.NewAckRequest("", inviteRequest, res, "", nil)
			ackRequest.AppendHeader(globalContactAddress.AsContactHeader())
			//手动替换ack请求目标地址, answer的contact可能不对.
			recipient := ackRequest.Recipient()
			recipient.SetHost(Config.PublicIP)
			recipient.SetPort(&Config.SipPort)

			Sugar.Infof("send ack %s", ackRequest.String())

			err := SipUA.Send(ackRequest)
			if err != nil {
				cancel()
				Sugar.Errorf("send ack error %s %s", err.Error(), ackRequest.String())
			} else {
				inviteOk = true
				bye = ackRequest.Clone().(sip.Request)
				bye.SetMethod(sip.BYE)
				bye.RemoveHeader("Via")
				if seq, ok := bye.CSeq(); ok {
					seq.SeqNo++
					seq.MethodName = sip.BYE
				}
			}
		} else if res.StatusCode() > 299 {
			cancel()
		}
	}))

	if !inviteOk {
		return
	}

	if stream.waitPublishStream() {
		stream.ByeRequest = bye
		w.WriteHeader(http.StatusOK)
	} else {
		SipUA.SendRequest(bye)
	}
}

func (api *ApiServer) CloseStream(streamId string) {
	stream, _ := StreamManager.Remove(streamId)
	if stream != nil && stream.ByeRequest != nil {
		SipUA.SendRequest(stream.ByeRequest)
		return
	}
}

func (api *ApiServer) OnPlayDone(streamId, protocol string, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("play done. protocol:%s stream id:%s", protocol, streamId)
	w.WriteHeader(http.StatusOK)
}

func (api *ApiServer) OnPublish(streamId, protocol string, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("publish. protocol:%s stream id:%s", protocol, streamId)

	w.WriteHeader(http.StatusOK)
	stream := StreamManager.Find(streamId)
	if stream != nil {
		stream.publishEvent <- 0
	}
}

func (api *ApiServer) OnPublishDone(streamId, protocol string, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("publish done. protocol:%s stream id:%s", protocol, streamId)

	w.WriteHeader(http.StatusOK)
	api.CloseStream(streamId)
}

func (api *ApiServer) OnDeviceList(w http.ResponseWriter, r *http.Request) {
	devices := DeviceManager.AllDevices()
	httpResponse2(w, devices)
}

func (api *ApiServer) OnIdleTimeout(streamId string, protocol string, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("publish timeout. protocol:%s stream id:%s", protocol, streamId)

	if protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		api.CloseStream(streamId)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (api *ApiServer) OnReceiveTimeout(streamId string, protocol string, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("receive timeout. protocol:%s stream id:%s", protocol, streamId)

	if protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		api.CloseStream(streamId)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
