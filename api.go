package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lkmio/avformat/librtp"
	"github.com/lkmio/avformat/utils"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ApiServer struct {
	router   *mux.Router
	upgrader *websocket.Upgrader
}

type InviteParams struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Setup     string `json:"setup"`
	Speed     string `json:"speed"`
	streamId  string
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

func withHookParams(f func(streamId, protocol string, w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
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
	apiServer.router.HandleFunc("/api/v1/hook/on_play", withHookParams(apiServer.OnPlay))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", withHookParams(apiServer.OnPlayDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", withHookParams(apiServer.OnPublish))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", withHookParams(apiServer.OnPublishDone))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", withHookParams(apiServer.OnIdleTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", withHookParams(apiServer.OnReceiveTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", withHookParams(apiServer.OnReceiveTimeout))
	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	//统一处理live/playback/download请求
	apiServer.router.HandleFunc("/api/v1/{action}/start", apiServer.OnInvite)
	apiServer.router.HandleFunc("/api/v1/stream/close", apiServer.OnCloseStream) //释放流(实时/回放/下载), 以拉流计数为准, 如果没有客户端拉流, 不等流媒体服务通知空闲超时，立即释放流，否则(还有客户端拉流)不会释放。

	apiServer.router.HandleFunc("/api/v1/device/list", apiServer.OnDeviceList)         //查询在线设备
	apiServer.router.HandleFunc("/api/v1/record/list", apiServer.OnRecordList)         //查询录像列表
	apiServer.router.HandleFunc("/api/v1/position/sub", apiServer.OnSubscribePosition) //订阅移动位置
	apiServer.router.HandleFunc("/api/v1/playback/seek", apiServer.OnSeekPlayback)     //回放seek
	apiServer.router.HandleFunc("/api/v1/ptz/control", apiServer.OnPTZControl)         //云台控制

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

func generateStreamId(inviteType InviteType, deviceId, channelId string, startTime, endTime string) string {
	utils.Assert(channelId != "")

	var streamId []string
	if deviceId != "" {
		streamId = append(streamId, deviceId)
	}

	streamId = append(streamId, channelId)
	if InviteTypePlayback == inviteType {
		return strings.Join(streamId, "/") + ".playback" + "." + startTime + "." + endTime
	} else if InviteTypeDownload == inviteType {
		return strings.Join(streamId, "/") + ".download" + "." + startTime + "." + endTime
	}

	return strings.Join(streamId, "/")
}

func (api *ApiServer) OnInvite(w http.ResponseWriter, r *http.Request) {
	v := InviteParams{}
	err := HttpDecodeJSONBody(w, r, &v)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	action := strings.ToLower(vars["action"])
	if "playback" == action {
		apiServer.DoInvite(InviteTypePlayback, v, true, w, r)
	} else if "download" == action {
		apiServer.DoInvite(InviteTypeDownload, v, true, w, r)
	} else if "live" == action {
		apiServer.DoInvite(InviteTypeLive, v, true, w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (api *ApiServer) OnLiveStart(device *DBDevice, params InviteParams, streamId string, w http.ResponseWriter, r *http.Request) (sip.Request, bool, error) {
	dialog, b := device.Live(streamId, params.ChannelID, params.Setup)
	return dialog, b, nil
}

func (api *ApiServer) OnPlaybackStart(device *DBDevice, params InviteParams, streamId string, w http.ResponseWriter, r *http.Request) (sip.Request, bool, error) {
	startTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.StartTime, time.Local)
	if err != nil {
		Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), params.StartTime)
		return nil, false, err
	}

	endTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.EndTime, time.Local)
	if err != nil {
		Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), params.EndTime)
		return nil, false, err
	}

	startTimeSeconds := strconv.FormatInt(startTime.Unix(), 10)
	endTimeSeconds := strconv.FormatInt(endTime.Unix(), 10)
	dialog, b := device.Playback(streamId, params.ChannelID, startTimeSeconds, endTimeSeconds, params.Setup)
	return dialog, b, nil
}

func (api *ApiServer) OnDownloadStart(device *DBDevice, params InviteParams, streamId string, w http.ResponseWriter, r *http.Request) (sip.Request, bool, error) {
	startTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.StartTime, time.Local)
	if err != nil {
		Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), params.StartTime)
		return nil, false, err
	}

	endTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.EndTime, time.Local)
	if err != nil {
		Sugar.Errorf("解析开始时间失败 err:%s start_time:%s", err.Error(), params.EndTime)
		return nil, false, err
	}

	startTimeSeconds := strconv.FormatInt(startTime.Unix(), 10)
	endTimeSeconds := strconv.FormatInt(endTime.Unix(), 10)
	speed, _ := strconv.Atoi(params.Speed)
	speed = int(math.Min(4, float64(speed)))

	dialog, b := device.Download(streamId, params.ChannelID, params.StartTime, startTimeSeconds, endTimeSeconds, speed)
	return dialog, b, nil
}

// DoInvite 处理Invite请求
// @params sync 是否异步等待流媒体的publish事件(确认收到流), 目前请求流分两种方式，流媒体hook和http接口, hook方式同步等待确认收到流再应答, http接口直接应答成功。
func (api *ApiServer) DoInvite(inviteType InviteType, params InviteParams, sync bool, w http.ResponseWriter, r *http.Request) (*Stream, bool) {
	device := DeviceManager.Find(params.DeviceID)
	if device == nil {
		Sugar.Warnf("设备离线 id:%s", params.DeviceID)
		return nil, false
	}

	streamId := params.streamId
	if streamId == "" {
		streamId = generateStreamId(inviteType, device.Id, params.ChannelID, params.StartTime, params.EndTime)
	}
	stream := &Stream{
		Id:         streamId,
		Protocol:   "28181",
		StreamType: inviteType,
	}

	var inviteOK bool
	var publishOK bool
	defer func() {
		if !inviteOK {
			StreamManager.Remove(streamId)
			w.WriteHeader(http.StatusInternalServerError)
		} else if !publishOK {
			CloseStream(streamId)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			response := map[string]string{
				"stream_id": streamId,
			}

			httpResponseOK(w, response)
		}
	}()

	//如果添加Stream失败, 说明Stream已经存在
	if stream, ok := StreamManager.Add(stream); !ok {
		Sugar.Infof("stream %s 已经存在", streamId)
		inviteOK = true
		publishOK = true
		return stream, true
	}

	var dialog sip.Request
	var err error
	if InviteTypePlayback == inviteType {
		dialog, inviteOK, err = api.OnPlaybackStart(device, params, streamId, w, r)
	} else if InviteTypeDownload == inviteType {
		dialog, inviteOK, err = api.OnDownloadStart(device, params, streamId, w, r)
	} else {
		dialog, inviteOK, err = api.OnLiveStart(device, params, streamId, w, r)
	}

	if !inviteOK || err != nil {
		StreamManager.Remove(streamId)
		w.WriteHeader(http.StatusInternalServerError)
		return nil, false
	}

	stream.DialogRequest = dialog
	StreamManager.AddWithCallId(stream)

	//启动收流超时计时器
	wait := func() bool {
		ok := stream.WaitForPublishEvent(10)
		if !ok {
			Sugar.Infof("收流超时 发送bye请求...")
			CloseStream(streamId)
		}
		return ok
	}

	if sync {
		publishOK = true
		go wait()
	} else {
		publishOK = wait()
	}

	return stream, publishOK
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

	//跳过非国标拉流
	split := strings.Split(streamId, "/")
	if len(split) != 2 || len(split[0]) != 20 || len(split[1]) < 20 {
		w.WriteHeader(http.StatusOK)
		return
	}

	//已经存在，累加计数
	if stream := StreamManager.Find(streamId); stream != nil {
		stream.IncreaseSinkCount()
		w.WriteHeader(http.StatusOK)
		return
	}

	deviceId := split[0]  //deviceId
	channelId := split[1] //channelId
	if len(channelId) > 20 {
		channelId = channelId[:20]
	}

	query := r.URL.Query()
	params := InviteParams{
		DeviceID:  deviceId,
		ChannelID: channelId,
		StartTime: query.Get("start_time"),
		EndTime:   query.Get("end_time"),
		Setup:     strings.ToLower(query.Get("setup")),
		Speed:     query.Get("speed"),
		streamId:  streamId,
	}

	streamType := strings.ToLower(query.Get("stream_type"))
	var stream *Stream
	var ok bool
	if "playback" == streamType {
		stream, ok = api.DoInvite(InviteTypeLive, params, false, w, r)
	} else if "download" == streamType {
		stream, ok = api.DoInvite(InviteTypeDownload, params, false, w, r)
	} else {
		stream, ok = api.DoInvite(InviteTypeLive, params, false, w, r)
	}

	if ok {
		stream.IncreaseSinkCount()
	}
}

func (api *ApiServer) OnCloseStream(w http.ResponseWriter, r *http.Request) {
	v := struct {
		StreamID string `json:"stream_id"`
	}{}

	err := HttpDecodeJSONBody(w, r, &v)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	stream := StreamManager.Find(v.StreamID)
	if stream == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if stream.SinkCount() > 0 {
		return
	}

	CloseStream(v.StreamID)
}

func CloseStream(streamId string) {
	stream, _ := StreamManager.Remove(streamId)
	if stream != nil {
		stream.Close(true)
	}
}

func (api *ApiServer) OnPlayDone(streamId, protocol string, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("play done. protocol:%s stream id:%s", protocol, streamId)
	if stream := StreamManager.Find(streamId); stream != nil {
		stream.DecreaseSinkCount()
	}
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
	CloseStream(streamId)
}

func (api *ApiServer) OnIdleTimeout(streamId string, protocol string, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("publish timeout. protocol:%s stream id:%s", protocol, streamId)

	if protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		CloseStream(streamId)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (api *ApiServer) OnReceiveTimeout(streamId string, protocol string, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("receive timeout. protocol:%s stream id:%s", protocol, streamId)

	if protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		CloseStream(streamId)
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
	err = device.DoQueryRecordList(v.ChannelId, v.StartTime, v.EndTime, sn, v.Type_)
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
	v := struct {
		StreamId string `json:"stream_id"`
		Seconds  int    `json:"seconds"`
	}{}

	if err := HttpDecodeJSONBody(w, r, &v); err != nil {
		httpResponse2(w, err)
		return
	}

	stream := StreamManager.Find(v.StreamId)
	if stream == nil || stream.DialogRequest == nil {
		return
	}

	seekRequest := stream.CreateRequestFromDialog(sip.INFO)
	seq, _ := seekRequest.CSeq()
	body := fmt.Sprintf(SeekBodyFormat, seq.SeqNo, v.Seconds)
	seekRequest.SetBody(body, true)
	seekRequest.RemoveHeader(RtspMessageType.Name())
	seekRequest.AppendHeader(&RtspMessageType)

	SipUA.SendRequest(seekRequest)
	w.WriteHeader(http.StatusOK)
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
