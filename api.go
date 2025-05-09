package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lkmio/avformat/utils"
	"github.com/lkmio/rtp"
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
	streamId  StreamID
}

type StreamParams struct {
	Stream     StreamID `json:"stream"`      // Source
	Protocol   string   `json:"protocol"`    // 推拉流协议
	RemoteAddr string   `json:"remote_addr"` // peer地址
}

type PlayDoneParams struct {
	StreamParams
	Sink string `json:"sink"`
}

type QueryRecordParams struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	Timeout   int    `json:"timeout"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Type_     string `json:"type"`
}

type DeviceChannelID struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

type SeekParams struct {
	StreamId StreamID `json:"stream_id"`
	Seconds  int      `json:"seconds"`
}

type PlatformChannel struct {
	ServerID string      `json:"server_id"`
	Channels [][2]string `json:"channels"` //二维数组, 索引0-设备ID/索引1-通道ID
}

type BroadcastParams struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	RoomID    string `json:"room_id"`
	Type      int    `json:"type"`
}

type HangupParams struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	RoomID    string `json:"room_id"`
}

type RecordParams struct {
	StreamParams
	Path string `json:"path"`
}

type StreamIDParams struct {
	StreamID StreamID `json:"stream_id"`
}

type PageQuery struct {
	PageNumber *int        `json:"page_number"` // 页数
	PageSize   *int        `json:"page_size"`   // 每页大小
	TotalPages int         `json:"total_pages"` // 总页数
	TotalCount int         `json:"total_count"` // 总记录数
	Data       interface{} `json:"data"`
}

type PageQueryChannel struct {
	PageQuery
	DeviceID string `json:"device_id"`
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

func withDecodedParams[T any](f func(params T, w http.ResponseWriter, req *http.Request), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := HttpDecodeJSONBody(w, req, params); err != nil {
			Sugar.Errorf("处理http请求失败 err: %s path: %s", err.Error(), req.URL.Path)
			httpResponseError(w, err.Error())
			return
		}

		f(params.(T), w, req)
	}
}

func withDecodedParams2[T any](f func(params T, w http.ResponseWriter, req *http.Request), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		_ = HttpDecodeJSONBody(w, req, params)
		f(params.(T), w, req)
	}
}

func startApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", withDecodedParams(apiServer.OnPlay, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", withDecodedParams(apiServer.OnPlayDone, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", withDecodedParams(apiServer.OnPublish, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", withDecodedParams(apiServer.OnPublishDone, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", withDecodedParams(apiServer.OnIdleTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", withDecodedParams(apiServer.OnReceiveTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", withDecodedParams(apiServer.OnRecord, &RecordParams{}))

	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	// 统一处理live/playback/download请求
	apiServer.router.HandleFunc("/api/v1/{action}/start", withDecodedParams(apiServer.OnInvite, &InviteParams{}))
	// 关闭国标流. 如果是实时流, 等收流或空闲超时自行删除. 回放或下载流立即删除.
	apiServer.router.HandleFunc("/api/v1/stream/close", withDecodedParams(apiServer.OnCloseStream, &StreamIDParams{}))

	apiServer.router.HandleFunc("/api/v1/device/list", withDecodedParams2(apiServer.OnDeviceList, &PageQuery{}))              // 查询设备列表
	apiServer.router.HandleFunc("/api/v1/channel/list", withDecodedParams(apiServer.OnChannelList, &PageQueryChannel{}))      // 查询通道列表
	apiServer.router.HandleFunc("/api/v1/record/list", withDecodedParams(apiServer.OnRecordList, &QueryRecordParams{}))       // 查询录像列表
	apiServer.router.HandleFunc("/api/v1/position/sub", withDecodedParams(apiServer.OnSubscribePosition, &DeviceChannelID{})) // 订阅移动位置
	apiServer.router.HandleFunc("/api/v1/playback/seek", withDecodedParams(apiServer.OnSeekPlayback, &SeekParams{}))          // 回放seek
	apiServer.router.HandleFunc("/api/v1/ptz/control", apiServer.OnPTZControl)                                                // 云台控制

	apiServer.router.HandleFunc("/api/v1/platform/list", apiServer.OnPlatformList)                                                           // 级联设备列表
	apiServer.router.HandleFunc("/api/v1/platform/add", withDecodedParams(apiServer.OnPlatformAdd, &GBPlatformRecord{}))                     // 添加级联设备
	apiServer.router.HandleFunc("/api/v1/platform/remove", withDecodedParams(apiServer.OnPlatformRemove, &GBPlatformRecord{}))               // 删除级联设备
	apiServer.router.HandleFunc("/api/v1/platform/channel/bind", withDecodedParams(apiServer.OnPlatformChannelBind, &PlatformChannel{}))     // 级联绑定通道
	apiServer.router.HandleFunc("/api/v1/platform/channel/unbind", withDecodedParams(apiServer.OnPlatformChannelUnbind, &PlatformChannel{})) // 级联解绑通道

	apiServer.router.HandleFunc("/ws/v1/talk", apiServer.OnWSTalk)                                                                                   // 语音广播/对讲, 主讲音频传输链路
	apiServer.router.HandleFunc("/api/v1/broadcast/invite", withDecodedParams(apiServer.OnBroadcast, &BroadcastParams{Type: int(BroadcastTypeTCP)})) // 发起语音广播
	apiServer.router.HandleFunc("/api/v1/broadcast/hangup", withDecodedParams(apiServer.OnHangup, &HangupParams{}))                                  // 挂断广播会话
	apiServer.router.HandleFunc("/api/v1/talk", apiServer.OnTalk)                                                                                    // 语音对讲
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

func (api *ApiServer) OnPlay(params *StreamParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("播放事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// [注意]: windows上使用cmd/power shell推拉流如果要携带多个参数, 请用双引号将与号引起来("&")
	// session_id是为了同一个录像文件, 允许同时点播多个.当然如果实时流支持多路预览, 也是可以的.
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001
	//ffplay -i http://127.0.0.1:8080/34020000001320000001/34020000001310000001.flv?setup=passive
	//ffplay -i http://127.0.0.1:8080/34020000001320000001/34020000001310000001.m3u8?setup=passive
	//ffplay -i rtsp://test:123456@127.0.0.1/34020000001320000001/34020000001310000001?setup=passive

	// 回放示例
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001.session_id_0?setup=passive"&"stream_type=playback"&"start_time=2024-06-18T15:20:56"&"end_time=2024-06-18T15:25:56
	//ffplay -i rtmp://127.0.0.1/34020000001320000001/34020000001310000001.session_id_0?setup=passive&stream_type=playback&start_time=2024-06-18T15:20:56&end_time=2024-06-18T15:25:56

	// 跳过非国标拉流
	sourceStream := strings.Split(string(params.Stream), "/")
	if len(sourceStream) != 2 || len(sourceStream[0]) != 20 || len(sourceStream[1]) < 20 {
		Sugar.Infof("跳过非国标拉流 stream: %s", params.Stream)
		return
	}

	// 已经存在，累加计数
	if stream := StreamManager.Find(params.Stream); stream != nil {
		stream.IncreaseSinkCount()
		return
	}

	deviceId := sourceStream[0]
	channelId := sourceStream[1]
	if len(channelId) > 20 {
		channelId = channelId[:20]
	}

	// 发起invite的参数
	query := r.URL.Query()
	inviteParams := &InviteParams{
		DeviceID:  deviceId,
		ChannelID: channelId,
		StartTime: query.Get("start_time"),
		EndTime:   query.Get("end_time"),
		Setup:     strings.ToLower(query.Get("setup")),
		Speed:     query.Get("speed"),
		streamId:  params.Stream,
	}

	var code int
	var stream *Stream
	var err error
	streamType := strings.ToLower(query.Get("stream_type"))
	if "playback" == streamType {
		code, stream, err = api.DoInvite(InviteTypeLive, inviteParams, false, w, r)
	} else if "download" == streamType {
		code, stream, err = api.DoInvite(InviteTypeDownload, inviteParams, false, w, r)
	} else {
		code, stream, err = api.DoInvite(InviteTypeLive, inviteParams, false, w, r)
	}

	if err != nil {
		Sugar.Errorf("请求流失败 err: %s", err.Error())
		utils.Assert(http.StatusOK != code)
	} else if http.StatusOK == code {
		stream.IncreaseSinkCount()
	}

	w.WriteHeader(code)
}

func (api *ApiServer) OnPlayDone(params *PlayDoneParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("播放结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	stream := StreamManager.Find(params.Stream)
	if stream == nil {
		Sugar.Errorf("处理播放结束事件失败, stream不存在. id: %s", params.Stream)
		return
	}

	if 0 == stream.DecreaseSinkCount() && Config.AutoCloseOnIdle {
		CloseStream(params.Stream, true)
	}

	// 级联断开连接, 向上级发送Bye请求
	if params.Protocol == "gb_stream_forward" {
		sink := stream.RemoveForwardStreamSink(params.Sink)
		if sink == nil || sink.Dialog == nil {
			return
		}

		if platform := PlatformManager.FindPlatform(sink.ServerID); platform != nil {
			callID, _ := sink.Dialog.CallID()
			platform.CloseStream(callID.String(), true, false)
		}
	}
}

func (api *ApiServer) OnPublish(params *StreamParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("推流事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	stream := StreamManager.Find(params.Stream)
	if stream != nil {
		stream.publishEvent <- 0
	}
}

func (api *ApiServer) OnPublishDone(params *StreamParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("推流结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	CloseStream(params.Stream, false)
}

func (api *ApiServer) OnIdleTimeout(params *StreamParams, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("推流空闲超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp空闲超时, 返回非200应答, 删除会话
	if params.Protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		CloseStream(params.Stream, false)
	}
}

func (api *ApiServer) OnReceiveTimeout(params *StreamParams, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("收流超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp推流超时, 返回非200应答, 删除会话
	if params.Protocol != "rtmp" {
		w.WriteHeader(http.StatusForbidden)
		CloseStream(params.Stream, false)
	}
}

func (api *ApiServer) OnRecord(params *RecordParams, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("录制事件. protocol: %s stream: %s path:%s ", params.Protocol, params.Stream, params.Path)
}

func (api *ApiServer) OnInvite(v *InviteParams, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	action := strings.ToLower(vars["action"])

	var code int
	var stream *Stream
	var err error
	if "playback" == action {
		code, stream, err = apiServer.DoInvite(InviteTypePlayback, v, true, w, r)
	} else if "download" == action {
		code, stream, err = apiServer.DoInvite(InviteTypeDownload, v, true, w, r)
	} else if "live" == action {
		code, stream, err = apiServer.DoInvite(InviteTypeLive, v, true, w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if http.StatusOK != code {
		Sugar.Errorf("请求流失败 err: %s", err.Error())
		httpResponseError(w, err.Error())
	} else {
		// 返回stream id和拉流地址
		response := struct {
			Stream string   `json:"stream_id"`
			Urls   []string `json:"urls"`
		}{
			string(stream.ID),
			stream.urls,
		}
		httpResponseOK(w, response)
	}
}

// DoInvite 发起Invite请求
// @params sync 是否异步等待流媒体的publish事件(确认收到流), 目前请求流分两种方式，流媒体hook和http接口, hook方式同步等待确认收到流再应答, http接口直接应答成功。
func (api *ApiServer) DoInvite(inviteType InviteType, params *InviteParams, sync bool, w http.ResponseWriter, r *http.Request) (int, *Stream, error) {
	device := DeviceManager.Find(params.DeviceID)
	if device == nil {
		return http.StatusNotFound, nil, fmt.Errorf("设备离线 id: %s", params.DeviceID)
	}

	// 解析回放或下载的时间范围参数
	var startTimeSeconds string
	var endTimeSeconds string
	if InviteTypeLive != inviteType {
		startTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.StartTime, time.Local)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		endTime, err := time.ParseInLocation("2006-01-02t15:04:05", params.EndTime, time.Local)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		startTimeSeconds = strconv.FormatInt(startTime.Unix(), 10)
		endTimeSeconds = strconv.FormatInt(endTime.Unix(), 10)
	}

	if params.streamId == "" {
		params.streamId = GenerateStreamId(inviteType, device.GetID(), params.ChannelID, params.StartTime, params.EndTime)
	}

	// 解析回放或下载速度参数
	speed, _ := strconv.Atoi(params.Speed)
	speed = int(math.Min(4, float64(speed)))
	stream, err := device.(*Device).StartStream(inviteType, params.streamId, params.ChannelID, startTimeSeconds, endTimeSeconds, params.Setup, speed, sync)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, stream, nil
}

func (api *ApiServer) OnCloseStream(v *StreamIDParams, w http.ResponseWriter, r *http.Request) {
	stream := StreamManager.Find(v.StreamID)

	// 等空闲或收流超时会自动关闭
	if stream != nil && stream.GetSinkCount() < 1 {
		CloseStream(v.StreamID, true)
	}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnDeviceList(v *PageQuery, w http.ResponseWriter, r *http.Request) {
	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	devices, total, err := DB.QueryDevices(*v.PageNumber, *v.PageSize)
	if err != nil {
		Sugar.Errorf("查询设备列表失败 page number: %d, size: %d err: %s", *v.PageNumber, *v.PageSize, err.Error())
		httpResponseError(w, err.Error())
		return
	}

	query := PageQuery{
		PageNumber: v.PageNumber,
		PageSize:   v.PageSize,
		TotalCount: total,
		TotalPages: int(math.Ceil(float64(total) / float64(*v.PageSize))),
		Data:       devices,
	}

	httpResponseOK(w, query)
}

func (api *ApiServer) OnChannelList(v *PageQueryChannel, w http.ResponseWriter, r *http.Request) {
	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	channels, total, err := DB.QueryChannels(v.DeviceID, *v.PageNumber, *v.PageSize)
	if err != nil {
		Sugar.Errorf("查询设备列表失败 device: %s page number: %d, size: %d err: %s", v.DeviceID, *v.PageNumber, *v.PageSize, err.Error())
		httpResponseError(w, err.Error())
		return
	}

	query := PageQuery{
		PageNumber: v.PageNumber,
		PageSize:   v.PageSize,
		TotalCount: total,
		TotalPages: int(math.Ceil(float64(total) / float64(*v.PageSize))),
		Data:       channels,
	}

	httpResponseOK(w, query)
}

func (api *ApiServer) OnRecordList(v *QueryRecordParams, w http.ResponseWriter, r *http.Request) {
	device := DeviceManager.Find(v.DeviceID)
	if device == nil {
		httpResponseError(w, "设备离线")
		return
	}

	sn := GetSN()
	err := device.QueryRecord(v.ChannelID, v.StartTime, v.EndTime, sn, v.Type_)
	if err != nil {
		logger.Error("发送查询录像请求失败 err: %s", err.Error())
		httpResponseError(w, err.Error())
		return
	}

	// 设置查询超时时长
	timeout := int(math.Max(math.Min(5, float64(v.Timeout)), 60))
	withTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

	var recordList []RecordInfo
	SNManager.AddEvent(sn, func(data interface{}) {
		response := data.(*QueryRecordInfoResponse)

		if len(response.DeviceList.Devices) > 0 {
			recordList = append(recordList, response.DeviceList.Devices...)
		}

		// 所有记录响应完毕
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

func (api *ApiServer) OnSubscribePosition(v *DeviceChannelID, w http.ResponseWriter, r *http.Request) {
	device := DeviceManager.Find(v.DeviceID)
	if device == nil {
		httpResponseError(w, "设备离线")
		return
	}

	if err := device.SubscribePosition(v.ChannelID); err != nil {
		logger.Error("发送订阅位置请求失败 err: %s", err.Error())
		httpResponseError(w, err.Error())
		return
	}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnSeekPlayback(v *SeekParams, w http.ResponseWriter, r *http.Request) {
	stream := StreamManager.Find(v.StreamId)
	if stream == nil || stream.Dialog == nil {
		httpResponseError(w, "会话不存在")
		return
	}

	seekRequest := stream.CreateRequestFromDialog(sip.INFO)
	seq, _ := seekRequest.CSeq()
	body := fmt.Sprintf(SeekBodyFormat, seq.SeqNo, v.Seconds)
	seekRequest.SetBody(body, true)
	seekRequest.RemoveHeader(RtspMessageType.Name())
	seekRequest.AppendHeader(&RtspMessageType)

	SipUA.SendRequest(seekRequest)
	httpResponseOK(w, nil)
}

func (api *ApiServer) OnPTZControl(w http.ResponseWriter, r *http.Request) {

}

func (api *ApiServer) OnWSTalk(w http.ResponseWriter, r *http.Request) {
	conn, err := api.upgrader.Upgrade(w, r, nil)
	if err != nil {
		Sugar.Errorf("websocket头检查失败 err: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	roomId := utils.RandStringBytes(10)
	room := BroadcastManager.CreateRoom(roomId)
	response := MalformedRequest{200, "ok", map[string]string{
		"room_id": roomId,
	}}

	conn.WriteJSON(response)

	packet := make([]byte, 1500)
	muxer := rtp.NewMuxer(8, 0, 0xFFFFFFFF)

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
			muxer.Input(bytes[offset:offset+min], uint32(min), func() []byte {
				return packet[2:]
			}, func(data []byte) {
				binary.BigEndian.PutUint16(packet, uint16(len(data)))
				room.DispatchRtpPacket(packet[:2+len(data)])
			})

			n -= min
		}
	}

	Sugar.Infof("主讲websocket断开连接 room: %s", roomId)

	sessions := BroadcastManager.RemoveRoom(roomId)
	for _, session := range sessions {
		session.Close(true)
	}
}

func (api *ApiServer) OnHangup(v *HangupParams, w http.ResponseWriter, r *http.Request) {
	if session := BroadcastManager.Remove(GenerateSessionId(v.DeviceID, v.ChannelID)); session != nil {
		session.Close(true)
	}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnBroadcast(v *BroadcastParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("语音广播 %v", *v)

	var err error
	// 响应错误消息
	defer func() {
		if err != nil {
			Sugar.Errorf("广播失败 err: %s", err.Error())
			httpResponseError(w, err.Error())
		}
	}()

	device := DeviceManager.Find(v.DeviceID)
	if device == nil {
		err = fmt.Errorf("设备离线")
		return
	}

	broadcastRoom := BroadcastManager.FindRoom(v.RoomID)
	if broadcastRoom == nil {
		//err := fmt.Errorf("the room with id '%s' is not found", v.RoomID)
		err = fmt.Errorf("广播房间找不到. room: %s", v.RoomID)
		return
	}

	// 每个设备的广播唯一ID
	sessionId := GenerateSessionId(v.DeviceID, v.ChannelID)
	if BroadcastManager.Find(sessionId) != nil {
		err = fmt.Errorf("设备正在广播中. session: %s", sessionId)
		return
	}

	// 生成让下级应答时携带的ID
	sourceId := v.RoomID + utils.RandStringBytes(10)
	session := &BroadcastSession{
		SourceID:  sourceId,
		DeviceID:  v.DeviceID,
		ChannelID: v.ChannelID,
		RoomId:    v.RoomID,
		Type:      BroadcastType(v.Type),
	}

	if !BroadcastManager.AddSession(v.RoomID, session) {
		err = fmt.Errorf("设备正在广播中. session: %s", sessionId)
		return
	}

	cancel := r.Context()
	transaction := device.Broadcast(sourceId, v.ChannelID)
	responses := transaction.Responses()
	var ok bool
	select {
	case response := <-responses:
		if response == nil {
			err = fmt.Errorf("信令超时")
			break
		}

		if response.StatusCode() != http.StatusOK {
			err = fmt.Errorf("answer has a bad status code: %d response: %s", response.StatusCode(), response.String())
			break
		}

		// 不等下级的广播请求, 直接等Invite
		timeout, _ := context.WithTimeout(r.Context(), 10*time.Second)
		select {
		case <-timeout.Done():
			err = fmt.Errorf("invite超时. session: %s", session.Id())
			break
		case code := <-session.Answer:
			if http.StatusOK != code {
				err = fmt.Errorf("bad status code %d", code)
			} else {
				ok = true
			}
			break
		}
		break
	case <-cancel.Done():
		// 取消http请求
		Sugar.Warnf("广播失败, 取消http请求. session: %s", session.Id())
		break
	}

	if ok {
		httpResponseOK(w, nil)
	} else {
		BroadcastManager.Remove(sessionId)
	}
}

func (api *ApiServer) OnTalk(w http.ResponseWriter, r *http.Request) {
}

func (api *ApiServer) OnStarted(w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("lkm启动")

	streams := StreamManager.PopAll()
	for _, stream := range streams {
		stream.Close(true, false)
	}
}

func (api *ApiServer) OnPlatformAdd(v *GBPlatformRecord, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("添加级联设备 %v", *v)

	var err error
	// 响应错误消息
	defer func() {
		if err != nil {
			Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
			httpResponseError(w, err.Error())
		}
	}()

	if v.Username == "" {
		v.Username = Config.SipId
		Sugar.Infof("级联设备使用默认的ID")
	}

	if len(v.Username) != 20 {
		err = fmt.Errorf("用户名长度必须20位")
		return
	} else if len(v.SeverID) != 20 {
		err = fmt.Errorf("上级ID长度必须20位")
		return
	}

	var platform *GBPlatform
	v.CreateTime = strconv.FormatInt(time.Now().UnixMilli(), 10)
	v.Status = "OFF"
	platform, err = NewGBPlatform(v, SipUA)

	if err == nil {
		if err = DB.SavePlatform(v); err == nil {
			utils.Assert(PlatformManager.AddPlatform(platform))

			platform.Start()
			httpResponseOK(w, nil)
		}
	}
}

func (api *ApiServer) OnPlatformRemove(v *GBPlatformRecord, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("删除级联设备 %v", *v)

	if err := DB.DeletePlatform(v); err != nil {
		Sugar.Errorf("删除级联设备失败 err: %s", err.Error())
		httpResponseOK(w, err.Error())
		return
	}

	platform := PlatformManager.RemovePlatform(v.SeverID)
	if platform != nil {
		platform.Stop()
	}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnPlatformList(w http.ResponseWriter, r *http.Request) {
	platforms, err := DB.LoadPlatforms()
	if err != nil {
		Sugar.Errorf("查询级联设备列表失败 err: %s", err.Error())
	}

	httpResponseOK(w, platforms)
}

func (api *ApiServer) OnPlatformChannelBind(v *PlatformChannel, w http.ResponseWriter, r *http.Request) {
	platform := PlatformManager.FindPlatform(v.ServerID)

	if platform == nil {
		Sugar.Errorf("绑定通道失败, 级联设备不存在 device: %s", v.ServerID)
		httpResponseError(w, "级联设备不存在")
		return
	}

	// 级联功能，通道号必须唯一
	channels, err := DB.BindChannels(v.ServerID, v.Channels)
	if err != nil {
		Sugar.Errorf("绑定通道失败 err: %s", err.Error())
		httpResponseError(w, err.Error())
		return
	}

	httpResponseOK(w, channels)
}

func (api *ApiServer) OnPlatformChannelUnbind(v *PlatformChannel, w http.ResponseWriter, r *http.Request) {
	platform := PlatformManager.FindPlatform(v.ServerID)
	if platform == nil {
		Sugar.Errorf("解绑通道失败, 级联设备不存在 device: %s", v.ServerID)
		httpResponseError(w, "级联设备不存在")
		return
	}

	channels, err := DB.UnbindChannels(v.ServerID, v.Channels)
	if err != nil {
		Sugar.Errorf("解绑通道失败 err: %s", err.Error())
		httpResponseError(w, err.Error())
		return
	}

	httpResponseOK(w, channels)
}
