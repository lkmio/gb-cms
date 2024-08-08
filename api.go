package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lkmio/avformat/librtp"
	"github.com/lkmio/avformat/utils"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ApiServer struct {
	router   *mux.Router
	upgrader *websocket.Upgrader
}

var apiServer *ApiServer

func init() {
	apiServer = &ApiServer{
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},

		router: mux.NewRouter(),
	}
}

func withCheckParams(f func(streamId, protocol string, w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if "" != req.URL.RawQuery {
			Sugar.Infof("on request %s?%s", req.URL.Path, req.URL.RawQuery)
		}

		v := struct {
			Stream     string `json:"stream"`      //Stream id
			Protocol   string `json:"protocol"`    //推拉流协议
			RemoteAddr string `json:"remote_addr"` //peer地址
		}{}

		err := HttpDecodeJSONBody(w, req, &v)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		f(v.Stream, v.Protocol, w, req)
	}
}

func startApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", withCheckParams(apiServer.OnPlay))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", withCheckParams(apiServer.OnPlayDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", withCheckParams(apiServer.OnPublish))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", withCheckParams(apiServer.OnPublishDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", withCheckParams(apiServer.OnIdleTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", withCheckParams(apiServer.OnReceiveTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", withCheckParams(apiServer.OnReceiveTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	apiServer.router.HandleFunc("/api/v1/device/list", apiServer.OnDeviceList)         //查询在线设备
	apiServer.router.HandleFunc("/api/v1/record/list", apiServer.OnRecordList)         //查询录像列表
	apiServer.router.HandleFunc("/api/v1/position/sub", apiServer.OnSubscribePosition) //订阅移动位置
	apiServer.router.HandleFunc("/api/v1/playback/seek", apiServer.OnSeekPlayback)     //回放seek

	apiServer.router.HandleFunc("/api/v1/ptz/control", apiServer.OnPTZControl) //云台控制

	apiServer.router.HandleFunc("/ws/v1/talk", apiServer.OnWSTalk)                 //语音广播/对讲, 音频传输链路
	apiServer.router.HandleFunc("/api/v1/broadcast/invite", apiServer.OnBroadcast) //语音广播
	apiServer.router.HandleFunc("/api/v1/broadcast/hangup", apiServer.OnHangup)    //挂断广播会话
	apiServer.router.HandleFunc("/api/v1/talk", apiServer.OnTalk)                  //语音对讲
	apiServer.router.HandleFunc("/broadcast.html", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "./broadcast.html")
	})
	apiServer.router.HandleFunc("/g711.js", func(writer http.ResponseWriter, request *http.Request) {
		http.ServeFile(writer, request, "./g711.js")
	})

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

	//[注意]: windows上使用cmd/power shell推拉流如果要携带多个参数, 请用双引号将与号引起来("&")
	//session_id是为了同一个录像文件, 允许同时点播多个.当然如果实时流支持多路预览, 也是可以的.
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001
	//ffplay -i http://127.0.0.1:8080/34020000001320000001/34020000001310000001.flv?setup=passive
	//ffplay -i http://127.0.0.1:8080/34020000001320000001/34020000001310000001.m3u8?setup=passive
	//ffplay -i rtsp://test:123456@127.0.0.1/34020000001320000001/34020000001310000001?setup=passive

	//回放示例
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001.session_id_0?setup=passive"&"stream_type=playback"&"start_time=2024-06-18T15:20:56"&"end_time=2024-06-18T15:25:56
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001.session_id_0?setup=passive&stream_type=playback&start_time=2024-06-18T15:20:56&end_time=2024-06-18T15:25:56

	stream := StreamManager.Find(streamId)
	if stream != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	split := strings.Split(streamId, "/")
	if len(split) != 2 {
		w.WriteHeader(http.StatusOK)
		return
	}

	//跳过非国标拉流
	if len(split[0]) != 20 || len(split[1]) < 20 {
		w.WriteHeader(http.StatusOK)
		return
	}

	deviceId := split[0]  //deviceId
	channelId := split[1] //channelId
	device := DeviceManager.Find(deviceId)

	if len(channelId) > 20 {
		channelId = channelId[:20]
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

	query := r.URL.Query()
	setup := strings.ToLower(query.Get("setup"))
	streamType := strings.ToLower(query.Get("stream_type"))
	startTimeStr := strings.ToLower(query.Get("start_time"))
	endTimeStr := strings.ToLower(query.Get("end_time"))
	speedStr := strings.ToLower(query.Get("speed"))

	var startTimeSeconds string
	var endTimeSeconds string
	var err error
	var ssrc string
	if "playback" == streamType || "download" == streamType {
		startTime, err := time.ParseInLocation("2006-01-02t15:04:05", startTimeStr, time.Local)
		if err != nil {
			Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), startTimeStr)
			return
		}
		endTime, err := time.ParseInLocation("2006-01-02t15:04:05", endTimeStr, time.Local)
		if err != nil {
			Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), startTimeStr)
			return
		}

		startTimeSeconds = strconv.FormatInt(startTime.Unix(), 10)
		endTimeSeconds = strconv.FormatInt(endTime.Unix(), 10)
		ssrc = GetVodSSRC()
	} else {
		ssrc = GetLiveSSRC()
	}

	ssrcValue, _ := strconv.Atoi(ssrc)
	ip, port, err := CreateGBSource(streamId, setup, uint32(ssrcValue))
	if err != nil {
		Sugar.Errorf("创建GBSource失败 err:%s", err.Error())
		return
	}

	var inviteRequest sip.Request
	if "playback" == streamType {
		inviteRequest, err = device.BuildPlaybackRequest(channelId, ip, port, startTimeSeconds, endTimeSeconds, setup, ssrc)
	} else if "download" == streamType {
		speed, _ := strconv.Atoi(speedStr)
		speed = int(math.Min(4, float64(speed)))
		inviteRequest, err = device.BuildDownloadRequest(channelId, ip, port, startTimeSeconds, endTimeSeconds, setup, speed, ssrc)
	} else {
		inviteRequest, err = device.BuildLiveRequest(channelId, ip, port, setup, ssrc)
	}

	if err != nil {
		return
	}

	var bye sip.Request
	var answer string
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	SipUA.SendRequestWithContext(reqCtx, inviteRequest, gosip.WithResponseHandler(func(res sip.Response, request sip.Request) {
		if res.StatusCode() < 200 {

		} else if res.StatusCode() == 200 {
			answer = res.Body()
			ackRequest := sip.NewAckRequest("", inviteRequest, res, "", nil)
			ackRequest.AppendHeader(globalContactAddress.AsContactHeader())
			//手动替换ack请求目标地址, answer的contact可能不对.
			recipient := ackRequest.Recipient()
			remoteIP, remotePortStr, _ := net.SplitHostPort(device.RemoteAddr)
			remotePort, _ := strconv.Atoi(remotePortStr)
			sipPort := sip.Port(remotePort)
			recipient.SetHost(remoteIP)
			recipient.SetPort(&sipPort)

			Sugar.Infof("send ack %s", ackRequest.String())

			err := SipUA.Send(ackRequest)
			if err != nil {
				cancel()
				Sugar.Errorf("send ack error %s %s", err.Error(), ackRequest.String())
			} else {
				inviteOk = true
				bye = device.CreateByeRequestFromAnswer(res, false)
			}
		} else if res.StatusCode() > 299 {
			cancel()
		}
	}))

	if !inviteOk {
		return
	}

	if "active" == setup {
		parse, err := sdp.Parse(answer)
		if err != nil {
			inviteOk = false
			Sugar.Errorf("解析应答sdp失败 err:%s sdp:%s", err.Error(), answer)
			return
		}
		if parse.Video == nil || parse.Video.Port == 0 {
			inviteOk = false
			Sugar.Errorf("应答没有视频连接地址 sdp:%s", answer)
			return
		}

		addr := fmt.Sprintf("%s:%d", parse.Addr, parse.Video.Port)
		if err = ConnectGBSource(streamId, addr); err != nil {
			inviteOk = false
			Sugar.Errorf("设置GB28181连接地址失败 err:%s addr:%s", err.Error(), addr)
		}
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
	if stream != nil {
		stream.Close(true)
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

func (api *ApiServer) OnRecord(streamId string, protocol string, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("receive onrecord. protocol:%s stream id:%s", protocol, streamId)
	w.WriteHeader(http.StatusOK)
}

func (api *ApiServer) OnDeviceList(w http.ResponseWriter, r *http.Request) {
	devices := DeviceManager.AllDevices()
	httpResponseOK(w, devices)
}

func (api *ApiServer) OnRecordList(w http.ResponseWriter, r *http.Request) {
	v := struct {
		DeviceId  string `json:"device_id"`
		ChannelId string `json:"channel_id"`
		Timeout   int    `json:"timeout"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Type_     string `json:"type"`
	}{}

	err := HttpDecodeJSONBody(w, r, &v)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	device := DeviceManager.Find(v.DeviceId)
	if device == nil {
		httpResponseOK(w, "设备离线")
		return
	}

	sn := GetSN()
	err = device.DoRecordList(v.ChannelId, v.StartTime, v.EndTime, sn, v.Type_)
	if err != nil {
		httpResponseOK(w, fmt.Sprintf("发送查询录像记录失败 err:%s", err.Error()))
		return
	}

	var recordList []RecordInfo
	timeout := int(math.Max(math.Min(5, float64(v.Timeout)), 60))
	//设置查询超时时长
	withTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

	SNManager.AddEvent(sn, func(data interface{}) {
		response := data.(*QueryRecordInfoResponse)

		if len(response.DeviceList.Devices) > 0 {
			recordList = append(recordList, response.DeviceList.Devices...)
		}

		//查询完成
		if len(recordList) >= response.SumNum {
			cancelFunc()
		}
	})

	select {
	case _ = <-withTimeout.Done():
		break
	}

	httpResponseOK(w, recordList)
}

func (api *ApiServer) OnSubscribePosition(w http.ResponseWriter, r *http.Request) {
	v := struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
	}{}

	if err := HttpDecodeJSONBody(w, r, &v); err != nil {
		httpResponse2(w, err)
		return
	}

	device := DeviceManager.Find(v.DeviceID)
	if device == nil {
		return
	}

	if err := device.DoSubscribePosition(v.ChannelID); err != nil {

	}

	w.WriteHeader(http.StatusOK)
}

func (api *ApiServer) OnSeekPlayback(w http.ResponseWriter, r *http.Request) {
}

func (api *ApiServer) OnPTZControl(w http.ResponseWriter, r *http.Request) {
}

func (api *ApiServer) OnWSTalk(w http.ResponseWriter, r *http.Request) {
	conn, err := api.upgrader.Upgrade(w, r, nil)
	if err != nil {
		Sugar.Errorf("websocket头检查失败 err:%s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	roomId := utils.RandStringBytes(10)
	room := BroadcastManager.CreateRoom(roomId)
	response := MalformedRequest{200, "ok", map[string]string{
		"room_id": roomId,
	}}

	conn.WriteJSON(response)

	rtp := make([]byte, 1500)
	muxer := librtp.NewMuxer(8, 0, 0xFFFFFFFF)
	muxer.SetAllocHandler(func(params interface{}) []byte {
		return rtp[2:]
	})
	muxer.SetWriteHandler(func(data []byte, timestamp uint32, params interface{}) {
		binary.BigEndian.PutUint16(rtp, uint16(len(data)))
		room.DispatchRtpPacket(rtp[:2+len(data)])
	})

	for {
		_, bytes, err := conn.ReadMessage()
		n := len(bytes)
		if err != nil {
			Sugar.Infof("语音断开连接")
			break
		} else if n < 1 {
			continue
		}

		count := (n-1)/320 + 1
		for i := 0; i < count; i++ {
			offset := i * 320
			min := int(math.Min(float64(n), 320))
			muxer.Input(bytes[offset:offset+min], uint32(min))
			n -= min
		}
	}

	Sugar.Infof("主讲websocket断开连接 roomid:%s", roomId)
	muxer.Close()

	sessions := BroadcastManager.RemoveRoom(roomId)
	for _, session := range sessions {
		session.Close(true)
	}
}

func (api *ApiServer) OnHangup(w http.ResponseWriter, r *http.Request) {
	v := struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
		RoomID    string `json:"room_id"`
	}{}

	if err := HttpDecodeJSONBody(w, r, &v); err != nil {
		httpResponse2(w, err)
		return
	}

	if session := BroadcastManager.Remove(GenerateSessionId(v.DeviceID, v.ChannelID)); session != nil {
		session.Close(true)
	}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnBroadcast(w http.ResponseWriter, r *http.Request) {
	v := struct {
		DeviceID  string `json:"device_id"`
		ChannelID string `json:"channel_id"`
		RoomID    string `json:"room_id"`
		Type      int    `json:"type"`
	}{
		Type: int(BroadcastTypeTCP),
	}

	if err := HttpDecodeJSONBody(w, r, &v); err != nil {
		httpResponse2(w, err)
		return
	}

	broadcastRoom := BroadcastManager.FindRoom(v.RoomID)
	if broadcastRoom == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	//全局唯一ID
	sessionId := GenerateSessionId(v.DeviceID, v.ChannelID)
	if BroadcastManager.Find(sessionId) != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	device := DeviceManager.Find(v.DeviceID)
	if device == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	sourceId := v.RoomID + utils.RandStringBytes(10)
	session := &BroadcastSession{
		SourceID:  sourceId,
		DeviceID:  v.DeviceID,
		ChannelID: v.ChannelID,
		RoomId:    v.RoomID,
		Type:      BroadcastType(v.Type),
	}

	if BroadcastManager.AddSession(v.RoomID, session) {
		device.DoBroadcast(sourceId, v.ChannelID)
		httpResponseOK(w, nil)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}

	select {
	case <-session.Answer:
		break
	case <-r.Context().Done():
		break
	}

	if !session.Successful {
		Sugar.Errorf("广播失败 session:%s", sessionId)
		BroadcastManager.Remove(sessionId)
	} else {
		Sugar.Infof("广播成功 session:%s", sessionId)
	}
}

func (api *ApiServer) OnTalk(w http.ResponseWriter, r *http.Request) {
}

func (api *ApiServer) OnStarted(w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("lkm启动")

	streams := StreamManager.PopAll()
	for _, stream := range streams {
		stream.Close(true)
	}
}
