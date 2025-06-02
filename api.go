package main

import (
	"context"
	"fmt"
	"gb-cms/hook"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	streamId  StreamID
}

type StreamParams struct {
	Stream     StreamID `json:"stream"`      // Source
	Protocol   int      `json:"protocol"`    // 推拉流协议
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
	ServerAddr string      `json:"server_addr"`
	Channels   [][2]string `json:"channels"` //二维数组, 索引0-设备ID/索引1-通道ID
}

type BroadcastParams struct {
	DeviceID  string     `json:"device_id"`
	ChannelID string     `json:"channel_id"`
	StreamId  StreamID   `json:"stream_id"`
	Setup     *SetupType `json:"setup"`
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
	GroupID  string `json:"group_id"`
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

func withJsonParams[T any](f func(params T, w http.ResponseWriter, req *http.Request), params T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := HttpDecodeJSONBody(w, req, params); err != nil {
			Sugar.Errorf("解析请求体失败 err: %s path: %s", err.Error(), req.URL.Path)
			httpResponseError(w, err.Error())
			return
		}

		f(params, w, req)
	}
}

func withJsonResponse[T any](f func(params T, w http.ResponseWriter, req *http.Request) (interface{}, error), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		newParams := new(T)
		if err := HttpDecodeJSONBody(w, req, newParams); err != nil {
			Sugar.Errorf("解析请求体失败 err: %s path: %s", err.Error(), req.URL.Path)
			httpResponseError(w, err.Error())
			return
		}

		responseBody, err := f(*newParams, w, req)
		if err != nil {
			httpResponseError(w, err.Error())
		} else {
			httpResponseOK(w, responseBody)
		}
	}
}

func startApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", withJsonParams(apiServer.OnPlay, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", withJsonParams(apiServer.OnPlayDone, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", withJsonParams(apiServer.OnPublish, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", withJsonParams(apiServer.OnPublishDone, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", withJsonParams(apiServer.OnIdleTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", withJsonParams(apiServer.OnReceiveTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", withJsonParams(apiServer.OnRecord, &RecordParams{}))

	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	// 统一处理live/playback/download请求
	apiServer.router.HandleFunc("/api/v1/{action}/start", withJsonParams(apiServer.OnInvite, &InviteParams{}))
	// 关闭国标流. 如果是实时流, 等收流或空闲超时自行删除. 回放或下载流立即删除.
	apiServer.router.HandleFunc("/api/v1/stream/close", withJsonParams(apiServer.OnCloseStream, &StreamIDParams{}))

	apiServer.router.HandleFunc("/api/v1/device/list", withJsonResponse(apiServer.OnDeviceList, &PageQuery{}))               // 查询设备列表
	apiServer.router.HandleFunc("/api/v1/channel/list", withJsonResponse(apiServer.OnChannelList, &PageQueryChannel{}))      // 查询通道列表
	apiServer.router.HandleFunc("/api/v1/record/list", withJsonResponse(apiServer.OnRecordList, &QueryRecordParams{}))       // 查询录像列表
	apiServer.router.HandleFunc("/api/v1/position/sub", withJsonResponse(apiServer.OnSubscribePosition, &DeviceChannelID{})) // 订阅移动位置
	apiServer.router.HandleFunc("/api/v1/playback/seek", withJsonResponse(apiServer.OnSeekPlayback, &SeekParams{}))          // 回放seek
	apiServer.router.HandleFunc("/api/v1/ptz/control", apiServer.OnPTZControl)                                               // 云台控制

	apiServer.router.HandleFunc("/api/v1/platform/list", apiServer.OnPlatformList)                                                          // 级联设备列表
	apiServer.router.HandleFunc("/api/v1/platform/add", withJsonResponse(apiServer.OnPlatformAdd, &PlatformModel{}))                        // 添加级联设备
	apiServer.router.HandleFunc("/api/v1/platform/remove", withJsonResponse(apiServer.OnPlatformRemove, &PlatformModel{}))                  // 删除级联设备
	apiServer.router.HandleFunc("/api/v1/platform/channel/bind", withJsonResponse(apiServer.OnPlatformChannelBind, &PlatformChannel{}))     // 级联绑定通道
	apiServer.router.HandleFunc("/api/v1/platform/channel/unbind", withJsonResponse(apiServer.OnPlatformChannelUnbind, &PlatformChannel{})) // 级联解绑通道

	apiServer.router.HandleFunc("/api/v1/broadcast/invite", withJsonResponse(apiServer.OnBroadcast, &BroadcastParams{Setup: &DefaultSetupType})) // 发起语音广播
	apiServer.router.HandleFunc("/api/v1/broadcast/hangup", withJsonResponse(apiServer.OnHangup, &BroadcastParams{}))                            // 挂断广播会话
	apiServer.router.HandleFunc("/api/v1/talk", apiServer.OnTalk)                                                                                // 语音对讲

	apiServer.router.HandleFunc("/api/v1/jt/device/add", withJsonResponse(apiServer.OnVirtualDeviceAdd, &JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/edit", withJsonResponse(apiServer.OnVirtualDeviceEdit, &JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/remove", withJsonResponse(apiServer.OnVirtualDeviceRemove, &JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/list", withJsonResponse(apiServer.OnVirtualDeviceList, &PageQuery{}))

	apiServer.router.HandleFunc("/api/v1/jt/channel/add", withJsonResponse(apiServer.OnVirtualChannelAdd, &Channel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/edit", withJsonResponse(apiServer.OnVirtualChannelEdit, &Channel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/remove", withJsonResponse(apiServer.OnVirtualChannelRemove, &Channel{}))

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

	// 拉流地址携带的参数
	query := r.URL.Query()
	jtSource := query.Get("forward_type") == "gateway_1078"

	// 跳过非国标拉流
	sourceStream := strings.Split(string(params.Stream), "/")
	if !jtSource && (len(sourceStream) != 2 || len(sourceStream[0]) != 20 || len(sourceStream[1]) < 20) {
		Sugar.Infof("跳过非国标拉流 stream: %s", params.Stream)
		return
	}

	// 已经存在，累加计数
	if stream, _ := StreamDao.QueryStream(params.Stream); stream != nil {
		stream.IncreaseSinkCount()
		return
	}

	deviceId := sourceStream[0]
	channelId := sourceStream[1]
	if len(channelId) > 20 {
		channelId = channelId[:20]
	}

	var code int
	// 通知1078信令服务器
	if jtSource {
		if len(sourceStream) != 2 {
			code = http.StatusBadRequest
			Sugar.Errorf("1078信令服务器转发请求参数错误")
			return
		}

		simNumber := sourceStream[0]
		channelNumber := sourceStream[1]
		response, err := hook.PostOnInviteEvent(simNumber, channelNumber)
		if err != nil {
			code = http.StatusInternalServerError
			Sugar.Errorf("通知1078信令服务器失败 err: %s sim number: %s channel number: %s", err.Error(), simNumber, channelNumber)
		} else if code = response.StatusCode; code != http.StatusOK {
			Sugar.Errorf("通知1078信令服务器失败. 响应状态码: %d sim number: %s channel number: %s", response.StatusCode, simNumber, channelNumber)
		}
	} else {
		inviteParams := &InviteParams{
			DeviceID:  deviceId,
			ChannelID: channelId,
			StartTime: query.Get("start_time"),
			EndTime:   query.Get("end_time"),
			Setup:     strings.ToLower(query.Get("setup")),
			Speed:     query.Get("speed"),
			streamId:  params.Stream,
		}

		var stream *Stream
		var err error
		streamType := strings.ToLower(query.Get("stream_type"))
		if "playback" == streamType {
			code, stream, err = api.DoInvite(InviteTypePlay, inviteParams, false, w, r)
		} else if "download" == streamType {
			code, stream, err = api.DoInvite(InviteTypeDownload, inviteParams, false, w, r)
		} else {
			code, stream, err = api.DoInvite(InviteTypePlay, inviteParams, false, w, r)
		}

		if err != nil {
			Sugar.Errorf("请求流失败 err: %s", err.Error())
			utils.Assert(http.StatusOK != code)
		} else if http.StatusOK == code {
			stream.IncreaseSinkCount()
		}
	}

	w.WriteHeader(code)
}

func (api *ApiServer) OnPlayDone(params *PlayDoneParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("播放结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	sink := RemoveForwardSink(params.Stream, params.Sink)
	if sink == nil {
		return
	}

	// 级联断开连接, 向上级发送Bye请求
	if params.Protocol == TransStreamGBCascaded {
		if platform := PlatformManager.Find(sink.ServerAddr); platform != nil {
			callID, _ := sink.Dialog.CallID()
			platform.(*Platform).CloseStream(callID.Value(), true, false)
		}
	} else {
		sink.Close(true, false)
	}
}

func (api *ApiServer) OnPublish(params *StreamParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("推流事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	if SourceTypeRtmp == params.Protocol {
		return
	}

	stream := EarlyDialogs.Find(string(params.Stream))
	if stream != nil {
		stream.Put(200)
	}

	// 创建stream
	if params.Protocol == SourceTypeGBTalk || params.Protocol == SourceType1078 {
		s := &Stream{
			DeviceID:  params.Stream.DeviceID(),
			ChannelID: params.Stream.ChannelID(),
			StreamID:  params.Stream,
			Protocol:  params.Protocol,
		}

		_, ok := StreamDao.SaveStream(s)
		if !ok {
			Sugar.Errorf("处理推流事件失败, stream已存在. id: %s", params.Stream)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
}

func (api *ApiServer) OnPublishDone(params *StreamParams, w http.ResponseWriter, r *http.Request) {
	Sugar.Infof("推流结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	CloseStream(params.Stream, false)
	// 对讲websocket断开连接
	if SourceTypeGBTalk == params.Protocol {

	}
}

func (api *ApiServer) OnIdleTimeout(params *StreamParams, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("推流空闲超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp空闲超时, 返回非200应答, 删除会话
	if SourceTypeRtmp != params.Protocol {
		w.WriteHeader(http.StatusForbidden)
		CloseStream(params.Stream, false)
	}
}

func (api *ApiServer) OnReceiveTimeout(params *StreamParams, w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("收流超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp推流超时, 返回非200应答, 删除会话
	if SourceTypeRtmp != params.Protocol {
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
		code, stream, err = apiServer.DoInvite(InviteTypePlay, v, true, w, r)
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
			string(stream.StreamID),
			stream.urls,
		}
		httpResponseOK(w, response)
	}
}

// DoInvite 发起Invite请求
// @params sync 是否异步等待流媒体的publish事件(确认收到流), 目前请求流分两种方式，流媒体hook和http接口, hook方式同步等待确认收到流再应答, http接口直接应答成功。
func (api *ApiServer) DoInvite(inviteType InviteType, params *InviteParams, sync bool, w http.ResponseWriter, r *http.Request) (int, *Stream, error) {
	device, _ := DeviceDao.QueryDevice(params.DeviceID)
	if device == nil || !device.Online() {
		return http.StatusNotFound, nil, fmt.Errorf("设备离线 id: %s", params.DeviceID)
	}

	// 解析回放或下载的时间范围参数
	var startTimeSeconds string
	var endTimeSeconds string
	if InviteTypePlay != inviteType {
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
		params.streamId = GenerateStreamID(inviteType, device.GetID(), params.ChannelID, params.StartTime, params.EndTime)
	}

	// 解析回放或下载速度参数
	speed, _ := strconv.Atoi(params.Speed)
	speed = int(math.Min(4, float64(speed)))
	stream, err := device.StartStream(inviteType, params.streamId, params.ChannelID, startTimeSeconds, endTimeSeconds, params.Setup, speed, sync)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, stream, nil
}

func (api *ApiServer) OnCloseStream(v *StreamIDParams, w http.ResponseWriter, r *http.Request) {
	//stream := StreamManager.Find(v.StreamID)
	//
	//// 等空闲或收流超时会自动关闭
	//if stream != nil && stream.GetSinkCount() < 1 {
	//	CloseStream(v.StreamID, true)
	//}

	httpResponseOK(w, nil)
}

func (api *ApiServer) OnDeviceList(v *PageQuery, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("查询设备列表 %v", *v)

	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	devices, total, err := DeviceDao.QueryDevices(*v.PageNumber, *v.PageSize)
	if err != nil {
		Sugar.Errorf("查询设备列表失败 err: %s", err.Error())
		return nil, err
	}

	query := &PageQuery{
		PageNumber: v.PageNumber,
		PageSize:   v.PageSize,
		TotalCount: total,
		TotalPages: int(math.Ceil(float64(total) / float64(*v.PageSize))),
		Data:       devices,
	}

	return query, nil
}

func (api *ApiServer) OnChannelList(v *PageQueryChannel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("查询通道列表 %v", *v)

	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	channels, total, err := ChannelDao.QueryChannels(v.DeviceID, v.GroupID, *v.PageNumber, *v.PageSize)
	if err != nil {
		Sugar.Errorf("查询通道列表失败 err: %s", err.Error())
		return nil, err
	}

	query := &PageQuery{
		PageNumber: v.PageNumber,
		PageSize:   v.PageSize,
		TotalCount: total,
		TotalPages: int(math.Ceil(float64(total) / float64(*v.PageSize))),
		Data:       channels,
	}

	return query, nil
}

func (api *ApiServer) OnRecordList(v *QueryRecordParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("查询录像列表 %v", *v)

	device, _ := DeviceDao.QueryDevice(v.DeviceID)
	if device == nil || !device.Online() {
		Sugar.Errorf("查询录像列表失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	sn := GetSN()
	err := device.QueryRecord(v.ChannelID, v.StartTime, v.EndTime, sn, v.Type_)
	if err != nil {
		Sugar.Errorf("发送查询录像请求失败 err: %s", err.Error())
		return nil, err
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

	return recordList, nil
}

func (api *ApiServer) OnSubscribePosition(v *DeviceChannelID, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("订阅位置 %v", *v)

	device, _ := DeviceDao.QueryDevice(v.DeviceID)
	if device == nil || !device.Online() {
		Sugar.Errorf("订阅位置失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	if err := device.SubscribePosition(v.ChannelID); err != nil {
		Sugar.Errorf("订阅位置失败 err: %s", err.Error())
		return nil, err
	}

	return nil, nil
}

func (api *ApiServer) OnSeekPlayback(v *SeekParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("快进回放 %v", *v)

	stream, _ := StreamDao.QueryStream(v.StreamId)
	if stream == nil || stream.Dialog == nil {
		Sugar.Infof("快进回放失败 stream不存在 %s", v.StreamId)
		return nil, fmt.Errorf("stream不存在")
	}

	seekRequest := stream.CreateRequestFromDialog(sip.INFO)
	seq, _ := seekRequest.CSeq()
	body := fmt.Sprintf(SeekBodyFormat, seq.SeqNo, v.Seconds)
	seekRequest.SetBody(body, true)
	seekRequest.RemoveHeader(RtspMessageType.Name())
	seekRequest.AppendHeader(&RtspMessageType)

	SipStack.SendRequest(seekRequest)
	return nil, nil
}

func (api *ApiServer) OnPTZControl(w http.ResponseWriter, r *http.Request) {

}

func (api *ApiServer) OnHangup(v *BroadcastParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("广播挂断 %v", *v)

	id := GenerateStreamID(InviteTypeBroadcast, v.DeviceID, v.ChannelID, "", "")
	if sink := RemoveForwardSinkWithSinkStreamID(id); sink != nil {
		sink.Close(true, true)
	}

	return nil, nil
}

func (api *ApiServer) OnBroadcast(v *BroadcastParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("广播邀请 %v", *v)

	var sinkStreamId StreamID
	var InviteSourceId string
	var ok bool
	// 响应错误消息
	defer func() {
		if !ok {
			if InviteSourceId != "" {
				EarlyDialogs.Remove(InviteSourceId)
			}

			if sinkStreamId != "" {
				_, _ = SinkDao.DeleteForwardSinkBySinkStreamID(sinkStreamId)
			}
		}
	}()

	device, _ := DeviceDao.QueryDevice(v.DeviceID)
	if device == nil || !device.Online() {
		Sugar.Errorf("广播失败, 设备离线, DeviceID: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	// 主讲人id
	source, _ := StreamDao.QueryStream(v.StreamId)
	if source == nil {
		Sugar.Errorf("广播失败, 找不到主讲人, stream: %s", v.StreamId)
		return nil, fmt.Errorf("找不到主讲人")
	}

	// 生成下级设备Invite请求携带的user
	// server用于区分是哪个设备的广播

	InviteSourceId = string(v.StreamId) + utils.RandStringBytes(10)
	// 每个设备的广播唯一ID
	sinkStreamId = GenerateStreamID(InviteTypeBroadcast, v.DeviceID, v.ChannelID, "", "")

	setupType := SetupTypePassive
	if v.Setup != nil && *v.Setup >= SetupTypeUDP && *v.Setup <= SetupTypeActive {
		setupType = *v.Setup
	}

	sink := &Sink{
		StreamID:     v.StreamId,
		SinkStreamID: sinkStreamId,
		Protocol:     "gb_talk",
		CreateTime:   time.Now().Unix(),
		SetupType:    setupType,
	}

	streamWaiting := &StreamWaiting{data: sink}
	if err := SinkDao.SaveForwardSink(v.StreamId, sink); err != nil {
		Sugar.Errorf("广播失败, 设备正在广播中. stream: %s", sinkStreamId)
		return nil, fmt.Errorf("设备正在广播中")
	} else if _, ok = EarlyDialogs.Add(InviteSourceId, streamWaiting); !ok {
		Sugar.Errorf("广播失败, id冲突. id: %s", InviteSourceId)
		return nil, fmt.Errorf("id冲突")
	}

	ok = false
	cancel := r.Context()
	transaction := device.Broadcast(InviteSourceId, v.ChannelID)
	responses := transaction.Responses()
	select {
	// 等待message broadcast的应答
	case response := <-responses:
		if response == nil {
			Sugar.Errorf("广播失败, 信令超时. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("信令超时")
		}

		if response.StatusCode() != http.StatusOK {
			Sugar.Errorf("广播失败, 错误响应, status code: %d", response.StatusCode())
			return nil, fmt.Errorf("错误响应 code: %d", response.StatusCode())
		}

		// 等待下级设备的Invite请求
		code := streamWaiting.Receive(10)
		if code == -1 {
			Sugar.Errorf("广播失败, 等待invite超时. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("等待invite超时")
		} else if http.StatusOK != code {
			Sugar.Errorf("广播失败, 下级设备invite失败. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("错误应答 code: %d", code)
		} else {
			//ok = AddForwardSink(v.StreamId, sink)
			ok = true
		}
		break
	case <-cancel.Done():
		// http请求取消
		Sugar.Warnf("广播失败, http请求取消. session: %s", sinkStreamId)
		break
	}

	return nil, nil
}

func (api *ApiServer) OnTalk(w http.ResponseWriter, r *http.Request) {

}

func (api *ApiServer) OnStarted(w http.ResponseWriter, req *http.Request) {
	Sugar.Infof("lkm启动")

	streams, _ := StreamDao.DeleteStreams()
	for _, stream := range streams {
		stream.Close(true, false)
	}

	sinks, _ := SinkDao.DeleteForwardSinks()
	for _, sink := range sinks {
		sink.Close(true, false)
	}
}

func (api *ApiServer) OnPlatformAdd(v *PlatformModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("添加级联设备 %v", *v)

	if v.Username == "" {
		v.Username = Config.SipID
		Sugar.Infof("级联设备使用本级域: %s", Config.SipID)
	}

	if len(v.Username) != 20 {
		err := fmt.Errorf("用户名长度必须20位")
		Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	} else if len(v.ServerID) != 20 {
		err := fmt.Errorf("上级ID长度必须20位")
		Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	}

	v.Status = "OFF"
	platform, err := NewPlatform(&v.SIPUAOptions, SipStack)
	if err != nil {
		Sugar.Errorf("创建级联设备失败 err: %s", err.Error())
		return nil, err
	}

	if !PlatformManager.Add(v.ServerAddr, platform) {
		Sugar.Errorf("ua添加失败, id冲突. key: %s", v.ServerAddr)
		return fmt.Errorf("ua添加失败, id冲突. key: %s", v.ServerAddr), nil
	} else if err = PlatformDao.SavePlatform(v); err != nil {
		PlatformManager.Remove(v.ServerAddr)
		Sugar.Errorf("保存级联设备失败 err: %s", err.Error())
		return nil, err
	}

	platform.Start()
	return nil, err
}

func (api *ApiServer) OnPlatformRemove(v *PlatformModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("删除级联设备 %v", *v)

	err := PlatformDao.DeleteUAByAddr(v.ServerAddr)
	if err != nil {
		return nil, err
	} else if platform := PlatformManager.Remove(v.ServerAddr); platform != nil {
		platform.Stop()
	}

	return nil, err
}

func (api *ApiServer) OnPlatformList(w http.ResponseWriter, r *http.Request) {
	//platforms := LoadPlatforms()
	//httpResponseOK(w, platforms)
}

func (api *ApiServer) OnPlatformChannelBind(v *PlatformChannel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("级联绑定通道 %v", *v)

	platform := PlatformManager.Find(v.ServerAddr)
	if platform == nil {
		Sugar.Errorf("绑定通道失败, 级联设备不存在 addr: %s", v.ServerAddr)
		return nil, fmt.Errorf("not found platform")
	}

	// 级联功能，通道号必须唯一
	channels, err := PlatformDao.BindChannels(v.ServerAddr, v.Channels)
	if err != nil {
		Sugar.Errorf("绑定通道失败 err: %s", err.Error())
		return nil, err
	}

	return channels, nil
}

func (api *ApiServer) OnPlatformChannelUnbind(v *PlatformChannel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("级联解绑通道 %v", *v)

	platform := PlatformManager.Find(v.ServerAddr)
	if platform == nil {
		Sugar.Errorf("解绑通道失败, 级联设备不存在 addr: %s", v.ServerAddr)
		return nil, fmt.Errorf("not found platform")
	}

	channels, err := PlatformDao.UnbindChannels(v.ServerAddr, v.Channels)
	if err != nil {
		Sugar.Errorf("解绑通道失败 err: %s", err.Error())
		return nil, err
	}

	return channels, nil
}
