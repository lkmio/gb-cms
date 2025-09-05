package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/hook"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/ghettovoice/gosip/sip"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lkmio/avformat/utils"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type ApiServer struct {
	router   *mux.Router
	upgrader *websocket.Upgrader
}

type InviteParams struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
	Setup     string `json:"setup"`
	Speed     string `json:"speed"`
	Token     string `json:"token"`
	streamId  common.StreamID
}

type StreamParams struct {
	Stream     common.StreamID `json:"stream"`      // Source
	Protocol   int             `json:"protocol"`    // 推拉流协议
	RemoteAddr string          `json:"remote_addr"` // peer地址
}

type PlayDoneParams struct {
	StreamParams
	Sink string `json:"sink"`
}

type QueryRecordParams struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	Timeout   int    `json:"timeout"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
	//Type_     string `json:"type"`
	Command string `json:"command"` // 云台控制命令 left/up/right/down/zoomin/zoomout
}

type DeviceChannelID struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

type SeekParams struct {
	StreamId common.StreamID `json:"stream_id"`
	Seconds  int             `json:"seconds"`
}

type PlatformChannel struct {
	ServerAddr string      `json:"server_addr"`
	Channels   [][2]string `json:"channels"` //二维数组, 索引0-设备ID/索引1-通道ID
}

type BroadcastParams struct {
	DeviceID  string            `json:"device_id"`
	ChannelID string            `json:"channel_id"`
	StreamId  common.StreamID   `json:"stream_id"`
	Setup     *common.SetupType `json:"setup"`
}

type RecordParams struct {
	StreamParams
	Path string `json:"path"`
}

type StreamIDParams struct {
	StreamID string `json:"streamid"`
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

type SetMediaTransportReq struct {
	DeviceID           string `json:"serial"`
	MediaTransport     string `json:"media_transport"`
	MediaTransportMode string `json:"media_transport_mode"`
}

// QueryDeviceChannel 查询设备和通道的参数
type QueryDeviceChannel struct {
	DeviceID    string `json:"serial"`
	GroupID     string `json:"dir_serial"`
	PCode       string `json:"pcode"`
	Start       int    `json:"start"`
	Limit       int    `json:"limit"`
	Keyword     string `json:"q"`
	Online      string `json:"online"`
	Enable      string `json:"enable"`
	ChannelType string `json:"channel_type"` // dir-查询子目录
	Order       string `json:"order"`        // asc/desc
	Sort        string `json:"sort"`         // Channel-根据数据库ID排序/iD-根据通道ID排序
	SMS         string `json:"sms"`
	Filter      string `json:"filter"`
}

type DeleteDevice struct {
	DeviceID string `json:"serial"`
	IP       string `json:"ip"`
	Forbid   bool   `json:"forbid"`
	UA       string `json:"ua"`
}

type SetEnable struct {
	ID              int  `json:"id"`
	Enable          bool `json:"enable"`
	ShareAllChannel bool `json:"shareallchannel"`
}

type QueryCascadeChannelList struct {
	QueryDeviceChannel
	ID      string `json:"id"`
	Related bool   `json:"related"` // 只看已选择
	Reverse bool   `json:"reverse"`
}

type ChannelListResult struct {
	ChannelCount int               `json:"ChannelCount"`
	ChannelList  []*LiveGBSChannel `json:"ChannelList"`
}

type CascadeChannel struct {
	CascadeID string
	*LiveGBSChannel
}

type CustomChannel struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	CustomID  string `json:"id"`
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

// 验证和刷新token
func withVerify(f func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ok := TokenManager.Refresh(cookie.Value, time.Now())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else if AdminMD5 == PwdMD5 && req.URL.Path != "/api/v1/modifypassword" && req.URL.Path != "/api/v1/userinfo" {
			// 如果没有修改默认密码, 只允许放行这2个接口
			return
		}

		f(w, req)
	}
}

func withVerify2(onSuccess func(w http.ResponseWriter, req *http.Request), onFailure func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err == nil && TokenManager.Refresh(cookie.Value, time.Now()) {
			onSuccess(w, req)
		} else {
			onFailure(w, req)
		}
	}
}

func startApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", common.WithJsonParams(apiServer.OnPlay, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", common.WithJsonParams(apiServer.OnPlayDone, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", common.WithJsonParams(apiServer.OnPublish, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", common.WithJsonParams(apiServer.OnPublishDone, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", common.WithJsonParams(apiServer.OnIdleTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", common.WithJsonParams(apiServer.OnReceiveTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", common.WithJsonParams(apiServer.OnRecord, &RecordParams{}))

	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	// 统一处理live/playback/download请求
	apiServer.router.HandleFunc("/api/v1/{action}/start", withVerify(common.WithFormDataParams(apiServer.OnInvite, InviteParams{})))
	// 关闭国标流. 如果是实时流, 等收流或空闲超时自行删除. 回放或下载流立即删除.
	apiServer.router.HandleFunc("/api/v1/stream/stop", withVerify(common.WithFormDataParams(apiServer.OnCloseStream, InviteParams{})))

	apiServer.router.HandleFunc("/api/v1/device/list", withVerify(common.WithQueryStringParams(apiServer.OnDeviceList, QueryDeviceChannel{})))                          // 查询设备列表
	apiServer.router.HandleFunc("/api/v1/device/channeltree", withVerify(common.WithQueryStringParams(apiServer.OnDeviceTree, QueryDeviceChannel{})))                   // 设备树
	apiServer.router.HandleFunc("/api/v1/device/channellist", withVerify(common.WithQueryStringParams(apiServer.OnChannelList, QueryDeviceChannel{})))                  // 查询通道列表
	apiServer.router.HandleFunc("/api/v1/device/fetchcatalog", withVerify(common.WithQueryStringParams(apiServer.OnCatalogQuery, QueryDeviceChannel{})))                // 更新通道
	apiServer.router.HandleFunc("/api/v1/device/remove", withVerify(common.WithFormDataParams(apiServer.OnDeviceRemove, DeleteDevice{})))                               // 删除设备
	apiServer.router.HandleFunc("/api/v1/device/setmediatransport", withVerify(common.WithFormDataParams(apiServer.OnDeviceMediaTransportSet, SetMediaTransportReq{}))) // 设置设备媒体传输模式

	apiServer.router.HandleFunc("/api/v1/playback/recordlist", withVerify(common.WithQueryStringParams(apiServer.OnRecordList, QueryRecordParams{}))) // 查询录像列表
	apiServer.router.HandleFunc("/api/v1/stream/info", withVerify(apiServer.OnStreamInfo))
	apiServer.router.HandleFunc("/api/v1/device/session/list", withVerify(common.WithQueryStringParams(apiServer.OnSessionList, QueryDeviceChannel{}))) // 推流列表
	apiServer.router.HandleFunc("/api/v1/device/session/stop", withVerify(common.WithFormDataParams(apiServer.OnSessionStop, StreamIDParams{})))        // 关闭流
	apiServer.router.HandleFunc("/api/v1/device/setchannelid", withVerify(common.WithFormDataParams(apiServer.OnCustomChannelSet, CustomChannel{})))    // 关闭流

	apiServer.router.HandleFunc("/api/v1/position/sub", common.WithJsonResponse(apiServer.OnSubscribePosition, &DeviceChannelID{}))        // 订阅移动位置
	apiServer.router.HandleFunc("/api/v1/playback/seek", common.WithJsonResponse(apiServer.OnSeekPlayback, &SeekParams{}))                 // 回放seek
	apiServer.router.HandleFunc("/api/v1/control/ptz", withVerify(common.WithFormDataParams(apiServer.OnPTZControl, QueryRecordParams{}))) // 云台控制

	apiServer.router.HandleFunc("/api/v1/cascade/list", withVerify(common.WithQueryStringParams(apiServer.OnPlatformList, QueryDeviceChannel{})))                    // 级联设备列表
	apiServer.router.HandleFunc("/api/v1/cascade/save", withVerify(common.WithFormDataParams(apiServer.OnPlatformAdd, LiveGBSCascade{})))                            // 添加级联设备
	apiServer.router.HandleFunc("/api/v1/cascade/setenable", withVerify(common.WithFormDataParams(apiServer.OnEnableSet, SetEnable{})))                              // 添加级联设备
	apiServer.router.HandleFunc("/api/v1/cascade/remove", withVerify(common.WithFormDataParams(apiServer.OnPlatformRemove, SetEnable{})))                            // 删除级联设备
	apiServer.router.HandleFunc("/api/v1/cascade/channellist", withVerify(common.WithQueryStringParams(apiServer.OnPlatformChannelList, QueryCascadeChannelList{}))) // 级联设备通道列表

	apiServer.router.HandleFunc("/api/v1/cascade/savechannels", withVerify(apiServer.OnPlatformChannelBind))                                           // 级联绑定通道
	apiServer.router.HandleFunc("/api/v1/cascade/removechannels", withVerify(apiServer.OnPlatformChannelUnbind))                                       // 级联解绑通道
	apiServer.router.HandleFunc("/api/v1/cascade/setshareallchannel", withVerify(common.WithFormDataParams(apiServer.OnShareAllChannel, SetEnable{}))) // 开启或取消级联所有通道
	apiServer.router.HandleFunc("/api/v1/cascade/pushcatalog", withVerify(common.WithFormDataParams(apiServer.OnCatalogPush, SetEnable{})))            // 推送目录

	// 暂未开发
	apiServer.router.HandleFunc("/api/v1/alarm/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))                // 报警查询
	apiServer.router.HandleFunc("/api/v1/cloudrecord/querychannels", withVerify(func(w http.ResponseWriter, req *http.Request) {})) // 云端录像
	apiServer.router.HandleFunc("/api/v1/user/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))                 // 用户管理
	apiServer.router.HandleFunc("/api/v1/log/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))                  // 操作日志

	apiServer.router.HandleFunc("/api/v1/broadcast/invite", common.WithJsonResponse(apiServer.OnBroadcast, &BroadcastParams{Setup: &common.DefaultSetupType})) // 发起语音广播
	apiServer.router.HandleFunc("/api/v1/broadcast/hangup", common.WithJsonResponse(apiServer.OnHangup, &BroadcastParams{}))                                   // 挂断广播会话
	apiServer.router.HandleFunc("/api/v1/talk", apiServer.OnTalk)                                                                                              // 语音对讲

	apiServer.router.HandleFunc("/api/v1/jt/device/add", common.WithJsonResponse(apiServer.OnVirtualDeviceAdd, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/edit", common.WithJsonResponse(apiServer.OnVirtualDeviceEdit, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/remove", common.WithJsonResponse(apiServer.OnVirtualDeviceRemove, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/list", common.WithJsonResponse(apiServer.OnVirtualDeviceList, &PageQuery{}))

	apiServer.router.HandleFunc("/api/v1/jt/channel/add", common.WithJsonResponse(apiServer.OnVirtualChannelAdd, &dao.ChannelModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/edit", common.WithJsonResponse(apiServer.OnVirtualChannelEdit, &dao.ChannelModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/remove", common.WithJsonResponse(apiServer.OnVirtualChannelRemove, &dao.ChannelModel{}))
	apiServer.router.HandleFunc("/logout", func(writer http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err == nil {
			TokenManager.Remove(cookie.Value)
			writer.Header().Set("Location", "/login.html")
			writer.WriteHeader(http.StatusFound)
			return
		}
	})

	registerLiveGBSApi()

	// 前端路由
	htmlRoot := "./html/"
	fileServer := http.FileServer(http.Dir(htmlRoot))
	apiServer.router.PathPrefix("/").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// 处理无扩展名的路径，自动添加.html扩展名
		path := request.URL.Path
		if !strings.Contains(path, ".") {
			// 检查是否存在对应的.html文件
			htmlPath := htmlRoot + path + ".html"
			if _, err := os.Stat(htmlPath); err == nil {
				// 如果存在对应的.html文件，则直接返回该文件
				http.ServeFile(writer, request, htmlPath)
				return
			}
		}

		// 供静态文件服务
		fileServer.ServeHTTP(writer, request)
	})

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

func (api *ApiServer) OnPlay(params *PlayDoneParams, w http.ResponseWriter, r *http.Request) {
	log.Sugar.Infof("播放事件. protocol: %s stream: %s", params.Protocol, params.Stream)

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
		log.Sugar.Infof("跳过非国标拉流 stream: %s", params.Stream)
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
			log.Sugar.Errorf("1078信令服务器转发请求参数错误")
			return
		}

		simNumber := sourceStream[0]
		channelNumber := sourceStream[1]
		response, err := hook.PostOnInviteEvent(simNumber, channelNumber)
		if err != nil {
			code = http.StatusInternalServerError
			log.Sugar.Errorf("通知1078信令服务器失败 err: %s sim number: %s channel number: %s", err.Error(), simNumber, channelNumber)
		} else if code = response.StatusCode; code != http.StatusOK {
			log.Sugar.Errorf("通知1078信令服务器失败. 响应状态码: %d sim number: %s channel number: %s", response.StatusCode, simNumber, channelNumber)
		}
	} else {
		// livegbs前端即使退出的播放，还是会拉流. 如果在hook中发起invite, 会造成不必要的请求.
		// 流不存在, 返回404
		if params.Protocol < stack.TransStreamGBCascaded {
			// 播放授权
			streamToken := query.Get("stream_token")
			if TokenManager.Find(streamToken) == nil {
				w.WriteHeader(http.StatusUnauthorized)
				log.Sugar.Errorf("播放鉴权失败, token不存在 token: %s", streamToken)
			} else if stream, _ := dao.Stream.QueryStream(params.Stream); stream == nil {
				w.WriteHeader(http.StatusNotFound)
			} else {
				_ = dao.Sink.SaveForwardSink(&dao.SinkModel{
					SinkID:     params.Sink,
					StreamID:   params.Stream,
					Protocol:   params.Protocol,
					RemoteAddr: params.RemoteAddr,
				})
			}
			return
		}

		// 对讲/级联, 在此处请求流
		inviteParams := &InviteParams{
			DeviceID:  deviceId,
			ChannelID: channelId,
			StartTime: query.Get("start_time"),
			EndTime:   query.Get("end_time"),
			Setup:     strings.ToLower(query.Get("setup")),
			Speed:     query.Get("speed"),
			streamId:  params.Stream,
		}

		var stream *dao.StreamModel
		var err error
		streamType := strings.ToLower(query.Get("stream_type"))
		if "playback" == streamType {
			code, stream, err = api.DoInvite(common.InviteTypePlay, inviteParams, false)
		} else if "download" == streamType {
			code, stream, err = api.DoInvite(common.InviteTypeDownload, inviteParams, false)
		} else {
			code, stream, err = api.DoInvite(common.InviteTypePlay, inviteParams, false)
		}

		if err != nil {
			log.Sugar.Errorf("请求流失败 err: %s", err.Error())
			utils.Assert(http.StatusOK != code)
		} else if http.StatusOK == code {
			_ = stream.ID

			_ = dao.Sink.SaveForwardSink(&dao.SinkModel{
				SinkID:     params.Sink,
				StreamID:   params.Stream,
				Protocol:   params.Protocol,
				RemoteAddr: params.RemoteAddr,
			})
		}
	}

	w.WriteHeader(code)
}

func (api *ApiServer) OnPlayDone(params *PlayDoneParams, _ http.ResponseWriter, _ *http.Request) {
	log.Sugar.Debugf("播放结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	sink, _ := dao.Sink.DeleteForwardSink(params.Sink)
	if sink == nil {
		return
	}

	// 级联断开连接, 向上级发送Bye请求
	if params.Protocol == stack.TransStreamGBCascaded {
		if platform := stack.PlatformManager.Find(sink.ServerAddr); platform != nil {
			callID, _ := sink.Dialog.CallID()
			platform.(*stack.Platform).CloseStream(callID.Value(), true, false)
		}
	} else {
		(&stack.Sink{sink}).Close(true, false)
	}
}

func (api *ApiServer) OnPublish(params *StreamParams, w http.ResponseWriter, _ *http.Request) {
	log.Sugar.Debugf("推流事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	if stack.SourceTypeRtmp == params.Protocol {
		return
	}

	stream := stack.EarlyDialogs.Find(string(params.Stream))
	if stream != nil {
		stream.Put(200)
	} else {
		log.Sugar.Infof("推流事件. 未找到stream. stream: %s", params.Stream)
	}

	// 创建stream
	if params.Protocol == stack.SourceTypeGBTalk || params.Protocol == stack.SourceType1078 {
		s := &dao.StreamModel{
			StreamID: params.Stream,
			Protocol: params.Protocol,
		}

		if params.Protocol != stack.SourceTypeGBTalk {
			s.DeviceID = params.Stream.DeviceID()
			s.ChannelID = params.Stream.ChannelID()
		}

		_, ok := dao.Stream.SaveStream(s)
		if !ok {
			log.Sugar.Errorf("处理推流事件失败, stream已存在. id: %s", params.Stream)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
}

func (api *ApiServer) OnPublishDone(params *StreamParams, _ http.ResponseWriter, _ *http.Request) {
	log.Sugar.Debugf("推流结束事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	//stack.CloseStream(params.Stream, false)
	//// 对讲websocket断开连接
	//if stack.SourceTypeGBTalk == params.Protocol {
	//
	//}
}

func (api *ApiServer) OnIdleTimeout(params *StreamParams, w http.ResponseWriter, _ *http.Request) {
	log.Sugar.Debugf("推流空闲超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp空闲超时, 返回非200应答, 删除会话
	if stack.SourceTypeRtmp != params.Protocol {
		w.WriteHeader(http.StatusForbidden)
		stack.CloseStream(params.Stream, false)
	}
}

func (api *ApiServer) OnReceiveTimeout(params *StreamParams, w http.ResponseWriter, _ *http.Request) {
	log.Sugar.Debugf("收流超时事件. protocol: %s stream: %s", params.Protocol, params.Stream)

	// 非rtmp推流超时, 返回非200应答, 删除会话
	if stack.SourceTypeRtmp != params.Protocol {
		w.WriteHeader(http.StatusForbidden)
		stack.CloseStream(params.Stream, false)
	}
}

func (api *ApiServer) OnRecord(params *RecordParams, _ http.ResponseWriter, _ *http.Request) {
	log.Sugar.Infof("录制事件. protocol: %s stream: %s path:%s ", params.Protocol, params.Stream, params.Path)
}

func (api *ApiServer) OnInvite(v *InviteParams, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	action := strings.ToLower(vars["action"])

	var code int
	var stream *dao.StreamModel
	var err error
	if "playback" == action {
		code, stream, err = apiServer.DoInvite(common.InviteTypePlayback, v, true)
	} else if "download" == action {
		code, stream, err = apiServer.DoInvite(common.InviteTypeDownload, v, true)
	} else if "stream" == action {
		code, stream, err = apiServer.DoInvite(common.InviteTypePlay, v, true)
	} else {
		return nil, fmt.Errorf("action not found")
	}

	if http.StatusOK != code {
		log.Sugar.Errorf("请求流失败 err: %s", err.Error())
		return nil, err
	}

	var urls map[string]string
	urls = make(map[string]string, 10)
	for _, url := range stream.Urls {
		var streamName string

		if strings.HasPrefix(url, "ws") {
			streamName = "WS_FLV"
		} else if strings.HasSuffix(url, ".flv") {
			streamName = "FLV"
		} else if strings.HasSuffix(url, ".m3u8") {
			streamName = "HLS"
		} else if strings.HasSuffix(url, ".rtc") {
			streamName = "WEBRTC"
		} else if strings.HasPrefix(url, "rtmp") {
			streamName = "RTMP"
		} else if strings.HasPrefix(url, "rtsp") {
			streamName = "RTSP"
		}

		// 加上登录的token, 播放授权
		url += "?stream_token=" + v.Token

		// 兼容livegbs前端播放webrtc
		if streamName == "WEBRTC" {
			if strings.HasPrefix(url, "http") {
				url = strings.Replace(url, "http", "webrtc", 1)
			} else if strings.HasPrefix(url, "https") {
				url = strings.Replace(url, "https", "webrtcs", 1)
			}

			url += "&wf=livegbs"
		}

		urls[streamName] = url
	}

	response := LiveGBSStream{
		AudioEnable:           false,
		CDN:                   "",
		CascadeSize:           0,
		ChannelID:             v.ChannelID,
		ChannelName:           "未读取通道名",
		ChannelPTZType:        0,
		CloudRecord:           false,
		DecodeSize:            0,
		DeviceID:              v.DeviceID,
		Duration:              1,
		FLV:                   urls["FLV"],
		HLS:                   urls["HLS"],
		InBitRate:             0,
		InBytes:               0,
		NumOutputs:            0,
		Ondemand:              true,
		OutBytes:              0,
		RTMP:                  urls["RTMP"],
		RecordStartAt:         "",
		RelaySize:             0,
		SMSID:                 "",
		SnapURL:               "",
		SourceAudioCodecName:  "",
		SourceAudioSampleRate: 0,
		SourceVideoCodecName:  "",
		SourceVideoFrameRate:  0,
		SourceVideoHeight:     0,
		SourceVideoWidth:      0,
		StartAt:               "",
		StreamID:              string(stream.StreamID),
		Transport:             "TCP",
		VideoFrameCount:       0,
		WEBRTC:                urls["WEBRTC"],
		WS_FLV:                urls["WS_FLV"],
	}

	return response, err
}

// DoInvite 发起Invite请求
// @params sync 是否异步等待流媒体的publish事件(确认收到流), 目前请求流分两种方式，流媒体hook和http接口, hook方式同步等待确认收到流再应答, http接口直接应答成功。
func (api *ApiServer) DoInvite(inviteType common.InviteType, params *InviteParams, sync bool) (int, *dao.StreamModel, error) {
	device, _ := dao.Device.QueryDevice(params.DeviceID)
	if device == nil || !device.Online() {
		return http.StatusNotFound, nil, fmt.Errorf("设备离线 id: %s", params.DeviceID)
	}

	// 解析回放或下载的时间范围参数
	var startTimeSeconds string
	var endTimeSeconds string
	if common.InviteTypePlay != inviteType {
		startTime, err := time.ParseInLocation("2006-01-02T15:04:05", params.StartTime, time.Local)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		endTime, err := time.ParseInLocation("2006-01-02T15:04:05", params.EndTime, time.Local)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		startTimeSeconds = strconv.FormatInt(startTime.Unix(), 10)
		endTimeSeconds = strconv.FormatInt(endTime.Unix(), 10)
	}

	if params.streamId == "" {
		params.streamId = common.GenerateStreamID(inviteType, device.GetID(), params.ChannelID, params.StartTime, params.EndTime)
	}

	if params.Setup == "" {
		params.Setup = device.Setup.String()
	}

	// 解析回放或下载速度参数
	speed, _ := strconv.Atoi(params.Speed)
	speed = int(math.Min(4, float64(speed)))
	d := stack.Device{device}
	stream, err := d.StartStream(inviteType, params.streamId, params.ChannelID, startTimeSeconds, endTimeSeconds, params.Setup, speed, sync)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, stream, nil
}

func (api *ApiServer) OnCloseStream(v *InviteParams, w http.ResponseWriter, _ *http.Request) (interface{}, error) {
	streamID := common.GenerateStreamID(common.InviteTypePlay, v.DeviceID, v.ChannelID, "", "")
	stack.CloseStream(streamID, true)
	return "OK", nil
}

func (api *ApiServer) OnDeviceList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := r.URL.Query()

	log.Sugar.Debugf("查询设备列表 %s", values.Encode())

	var status string
	if "" == q.Online {
	} else if "true" == q.Online {
		status = "ON"
	} else if "false" == q.Online {
		status = "OFF"
	}

	if "desc" != q.Order {
		q.Order = "asc"
	}

	devices, total, err := dao.Device.QueryDevices((q.Start/q.Limit)+1, q.Limit, status, q.Keyword, q.Order)
	if err != nil {
		log.Sugar.Errorf("查询设备列表失败 err: %s", err.Error())
		return nil, err
	}

	response := struct {
		DeviceCount int
		DeviceList_ []LiveGBSDevice `json:"DeviceList"`
	}{
		DeviceCount: total,
	}

	// livgbs设备离线后的最后心跳时间, 涉及到是否显示非法设备的批量删除按钮
	offlineTime := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05")
	for _, device := range devices {
		// 更新正在查询通道的进度
		var catalogProgress string
		data := stack.UniqueTaskManager.Find(stack.GenerateCatalogTaskID(device.GetID()))
		if data != nil {
			catalogSize := data.(*stack.CatalogProgress)

			if catalogSize.TotalSize > 0 {
				catalogProgress = fmt.Sprintf("%d/%d", catalogSize.RecvSize, catalogSize.TotalSize)
			}
		}

		var lastKeealiveTime string
		if device.Online() {
			lastKeealiveTime = device.LastHeartbeat.Format("2006-01-02 15:04:05")
		} else {
			lastKeealiveTime = offlineTime
		}

		response.DeviceList_ = append(response.DeviceList_, LiveGBSDevice{
			AlarmSubscribe:   false, // 报警订阅
			CatalogInterval:  3600,  // 目录刷新时间
			CatalogProgress:  catalogProgress,
			CatalogSubscribe: false, // 目录订阅
			ChannelCount:     device.ChannelsTotal,
			ChannelOverLoad:  false,
			Charset:          "GB2312",
			CivilCodeFirst:   false,
			CommandTransport: device.Transport,
			ContactIP:        "",
			CreatedAt:        device.CreatedAt.Format("2006-01-02 15:04:05"),
			CustomName:       "",
			DropChannelType:  "",
			GBVer:            "",
			ID:               device.GetID(),
			KeepOriginalTree: false,
			LastKeepaliveAt:  lastKeealiveTime,
			LastRegisterAt:   device.RegisterTime.Format("2006-01-02 15:04:05"),
			Latitude:         0,
			Longitude:        0,
			//Manufacturer:       device.Manufacturer,
			Manufacturer:       device.UserAgent,
			MediaTransport:     device.Setup.Transport(),
			MediaTransportMode: device.Setup.String(),
			Name:               device.Name,
			Online:             device.Online(),
			PTZSubscribe:       false, // PTZ订阅2022
			Password:           "",
			PositionSubscribe:  false, // 位置订阅
			RecordCenter:       false,
			RecordIndistinct:   false,
			RecvStreamIP:       "",
			RemoteIP:           device.RemoteIP,
			RemotePort:         device.RemotePort,
			RemoteRegion:       "",
			SMSGroupID:         "",
			SMSID:              "",
			StreamMode:         "",
			SubscribeInterval:  0,
			Type:               "GB",
			UpdatedAt:          device.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &response, nil
}

func (api *ApiServer) OnChannelList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	values := r.URL.Query()
	log.Sugar.Debugf("查询通道列表 %s", values.Encode())

	var deviceName string
	if q.DeviceID != "" {
		device, err := dao.Device.QueryDevice(q.DeviceID)
		if err != nil {
			log.Sugar.Errorf("查询设备失败 err: %s", err.Error())
			return nil, err
		}

		deviceName = device.Name
	}

	var status string
	if "" == q.Online {
	} else if "true" == q.Online {
		status = "ON"
	} else if "false" == q.Online {
		status = "OFF"
	}

	if "desc" != q.Order {
		q.Order = "asc"
	}

	channels, total, err := dao.Channel.QueryChannels(q.DeviceID, q.GroupID, (q.Start/q.Limit)+1, q.Limit, status, q.Keyword, q.Order, q.Sort, q.ChannelType == "dir")
	if err != nil {
		log.Sugar.Errorf("查询通道列表失败 err: %s", err.Error())
		return nil, err
	}

	response := ChannelListResult{
		ChannelCount: total,
	}

	index := q.Start + 1
	response.ChannelList = ChannelModels2LiveGBSChannels(index, channels, deviceName)
	return &response, nil
}

func (api *ApiServer) OnRecordList(v *QueryRecordParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("查询录像列表 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("查询录像列表失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	device := &stack.Device{model}
	sn := stack.GetSN()
	err := device.QueryRecord(v.ChannelID, v.StartTime, v.EndTime, sn, "all")
	if err != nil {
		log.Sugar.Errorf("发送查询录像请求失败 err: %s", err.Error())
		return nil, err
	}

	// 设置查询超时时长
	timeout := int(math.Max(math.Min(5, float64(v.Timeout)), 60))
	withTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

	var recordList []stack.RecordInfo
	stack.SNManager.AddEvent(sn, func(data interface{}) {
		response := data.(*stack.QueryRecordInfoResponse)

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

	response := struct {
		DeviceID   string
		Name       string
		RecordList []struct {
			DeviceID  string
			EndTime   string
			FileSize  uint64
			Name      string
			Secrecy   string
			StartTime string
			Type      string
		}
		SumNum int `json:"sumNum"`
	}{
		DeviceID: v.DeviceID,
		Name:     model.Name,
		SumNum:   len(recordList),
	}

	for _, record := range recordList {
		log.Sugar.Infof("查询录像列表 %v", record)
		response.RecordList = append(response.RecordList, struct {
			DeviceID  string
			EndTime   string
			FileSize  uint64
			Name      string
			Secrecy   string
			StartTime string
			Type      string
		}{
			DeviceID:  record.DeviceID,
			EndTime:   record.EndTime,
			FileSize:  record.FileSize,
			Name:      record.Name,
			Secrecy:   record.Secrecy,
			StartTime: record.StartTime,
			Type:      record.Type,
		})
	}

	return &response, nil
}

func (api *ApiServer) OnSubscribePosition(v *DeviceChannelID, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("订阅位置 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("订阅位置失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	device := &stack.Device{model}
	if err := device.SubscribePosition(v.ChannelID); err != nil {
		log.Sugar.Errorf("订阅位置失败 err: %s", err.Error())
		return nil, err
	}

	return nil, nil
}

func (api *ApiServer) OnSeekPlayback(v *SeekParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("快进回放 %v", *v)

	model, _ := dao.Stream.QueryStream(v.StreamId)
	if model == nil || model.Dialog == nil {
		log.Sugar.Errorf("快进回放失败 stream不存在 %s", v.StreamId)
		return nil, fmt.Errorf("stream不存在")
	}

	stream := &stack.Stream{model}
	seekRequest := stream.CreateRequestFromDialog(sip.INFO)
	seq, _ := seekRequest.CSeq()
	body := fmt.Sprintf(stack.SeekBodyFormat, seq.SeqNo, v.Seconds)
	seekRequest.SetBody(body, true)
	seekRequest.RemoveHeader(stack.RtspMessageType.Name())
	seekRequest.AppendHeader(&stack.RtspMessageType)

	common.SipStack.SendRequest(seekRequest)
	return nil, nil
}

func (api *ApiServer) OnPTZControl(v *QueryRecordParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("PTZ控制 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("PTZ控制失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	device := &stack.Device{model}
	device.ControlPTZ(v.Command, v.ChannelID)

	return "OK", nil
}

func (api *ApiServer) OnHangup(v *BroadcastParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("广播挂断 %v", *v)

	id := common.GenerateStreamID(common.InviteTypeBroadcast, v.DeviceID, v.ChannelID, "", "")
	if sink, _ := dao.Sink.DeleteForwardSinkBySinkStreamID(id); sink != nil {
		(&stack.Sink{sink}).Close(true, true)
	}

	return nil, nil
}

func (api *ApiServer) OnBroadcast(v *BroadcastParams, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("广播邀请 %v", *v)

	var sinkStreamId common.StreamID
	var InviteSourceId string
	var ok bool
	// 响应错误消息
	defer func() {
		if !ok {
			if InviteSourceId != "" {
				stack.EarlyDialogs.Remove(InviteSourceId)
			}

			if sinkStreamId != "" {
				_, _ = dao.Sink.DeleteForwardSinkBySinkStreamID(sinkStreamId)
			}
		}
	}()

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("广播失败, 设备离线, DeviceID: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	// 主讲人id
	stream, _ := dao.Stream.QueryStream(v.StreamId)
	if stream == nil {
		log.Sugar.Errorf("广播失败, 找不到主讲人, stream: %s", v.StreamId)
		return nil, fmt.Errorf("找不到主讲人")
	}

	// 生成下级设备Invite请求携带的user
	// server用于区分是哪个设备的广播

	InviteSourceId = string(v.StreamId) + utils.RandStringBytes(10)
	// 每个设备的广播唯一ID
	sinkStreamId = common.GenerateStreamID(common.InviteTypeBroadcast, v.DeviceID, v.ChannelID, "", "")

	setupType := common.SetupTypePassive
	if v.Setup != nil && *v.Setup >= common.SetupTypeUDP && *v.Setup <= common.SetupTypeActive {
		setupType = *v.Setup
	}

	sink := &dao.SinkModel{
		StreamID:     v.StreamId,
		SinkStreamID: sinkStreamId,
		Protocol:     stack.SourceTypeGBTalk,
		CreateTime:   time.Now().Unix(),
		SetupType:    setupType,
	}

	streamWaiting := &stack.StreamWaiting{Data: sink}
	if err := dao.Sink.SaveForwardSink(sink); err != nil {
		log.Sugar.Errorf("广播失败, 设备正在广播中. stream: %s", sinkStreamId)
		return nil, fmt.Errorf("设备正在广播中")
	} else if _, ok = stack.EarlyDialogs.Add(InviteSourceId, streamWaiting); !ok {
		log.Sugar.Errorf("广播失败, id冲突. id: %s", InviteSourceId)
		return nil, fmt.Errorf("id冲突")
	}

	ok = false
	cancel := r.Context()
	device := stack.Device{model}
	transaction := device.Broadcast(InviteSourceId, v.ChannelID)
	responses := transaction.Responses()
	select {
	// 等待message broadcast的应答
	case response := <-responses:
		if response == nil {
			log.Sugar.Errorf("广播失败, 信令超时. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("信令超时")
		}

		if response.StatusCode() != http.StatusOK {
			log.Sugar.Errorf("广播失败, 错误响应, status code: %d", response.StatusCode())
			return nil, fmt.Errorf("错误响应 code: %d", response.StatusCode())
		}

		// 等待下级设备的Invite请求
		code := streamWaiting.Receive(10)
		if code == -1 {
			log.Sugar.Errorf("广播失败, 等待invite超时. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("等待invite超时")
		} else if http.StatusOK != code {
			log.Sugar.Errorf("广播失败, 下级设备invite失败. stream: %s", sinkStreamId)
			return nil, fmt.Errorf("错误应答 code: %d", code)
		} else {
			//ok = AddForwardSink(v.StreamId, sink)
			ok = true
		}
		break
	case <-cancel.Done():
		// http请求取消
		log.Sugar.Warnf("广播失败, http请求取消. session: %s", sinkStreamId)
		break
	}

	return nil, nil
}

func (api *ApiServer) OnTalk(_ http.ResponseWriter, _ *http.Request) {

}

func (api *ApiServer) OnStarted(_ http.ResponseWriter, _ *http.Request) {
	log.Sugar.Infof("lkm启动")

	streams, _ := dao.Stream.DeleteStreams()
	for _, stream := range streams {
		(&stack.Stream{stream}).Close(true, false)
	}

	sinks, _ := dao.Sink.DeleteForwardSinks()
	for _, sink := range sinks {
		(&stack.Sink{sink}).Close(true, false)
	}
}

func (api *ApiServer) OnPlatformAdd(v *LiveGBSCascade, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("添加级联设备 %v", *v)

	if v.Username == "" {
		v.Username = common.Config.SipID
		log.Sugar.Infof("级联设备使用本级域: %s", common.Config.SipID)
	}

	var err error
	if len(v.Username) != 20 {
		err = fmt.Errorf("用户名长度必须20位")
		return nil, err
	} else if len(v.Serial) != 20 {
		err = fmt.Errorf("上级ID长度必须20位")
		return nil, err
	}

	if err != nil {
		log.Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	}

	v.Status = "OFF"
	model := dao.PlatformModel{
		SIPUAOptions: common.SIPUAOptions{
			Name:              v.Name,
			Username:          v.Username,
			Password:          v.Password,
			ServerID:          v.Serial,
			ServerAddr:        net.JoinHostPort(v.Host, strconv.Itoa(v.Port)),
			Transport:         v.CommandTransport,
			RegisterExpires:   v.RegisterInterval,
			KeepaliveInterval: v.KeepaliveInterval,
			Status:            common.OFF,
		},
	}

	platform, err := stack.NewPlatform(&model.SIPUAOptions, common.SipStack)
	if err != nil {
		return nil, err
	}

	// 编辑国标设备
	if v.ID != "" {
		// 停止旧的
		oldPlatform := stack.PlatformManager.Remove(model.ServerAddr)
		if oldPlatform != nil {
			oldPlatform.Stop()
		}

		// 更新数据库
		id, _ := strconv.ParseInt(v.ID, 10, 64)
		model.ID = uint(id)
		err = dao.Platform.UpdatePlatform(&model)
	} else {
		err = dao.Platform.SavePlatform(&model)
	}

	if err == nil && v.Enable {
		if !stack.PlatformManager.Add(model.ServerAddr, platform) {
			err = fmt.Errorf("地址冲突. key: %s", model.ServerAddr)
			if err != nil {
				_ = dao.Platform.DeletePlatformByAddr(model.ServerAddr)
			}
		} else {
			platform.Start()
		}
	}

	if err != nil {
		log.Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformRemove(v *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("删除级联设备 %v", *v)
	platform, _ := dao.Platform.QueryPlatformByID(v.ID)
	if platform == nil {
		return nil, fmt.Errorf("级联设备不存在")
	}

	_ = dao.Platform.DeletePlatformByID(v.ID)
	client := stack.PlatformManager.Remove(platform.ServerAddr)
	if client != nil {
		client.Stop()
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformList(q *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	response := struct {
		CascadeCount int               `json:"CascadeCount"`
		CascadeList  []*LiveGBSCascade `json:"CascadeList"`
	}{}

	platforms, total, err := dao.Platform.QueryPlatforms((q.Start/q.Limit)+1, q.Limit, q.Keyword, q.Enable, q.Online)
	if err == nil {
		response.CascadeCount = total
		for _, platform := range platforms {
			host, p, _ := net.SplitHostPort(platform.ServerAddr)
			port, _ := strconv.Atoi(p)
			response.CascadeList = append(response.CascadeList, &LiveGBSCascade{
				ID:                strconv.Itoa(int(platform.ID)),
				Enable:            platform.Enable,
				Name:              platform.Name,
				Serial:            platform.ServerID,
				Realm:             platform.ServerID[:10],
				Host:              host,
				Port:              port,
				LocalSerial:       platform.Username,
				Username:          platform.Username,
				Password:          platform.Password,
				Online:            platform.Status == common.ON,
				Status:            platform.Status,
				RegisterInterval:  platform.RegisterExpires,
				KeepaliveInterval: platform.KeepaliveInterval,
				CommandTransport:  platform.Transport,
				Charset:           "GB2312",
				CatalogGroupSize:  1,
				LoadLimit:         0,
				CivilCodeLimit:    8,
				DigestAlgorithm:   "",
				GM:                false,
				Cert:              "***",
				CreateAt:          platform.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdateAt:          platform.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return response, nil
}

func (api *ApiServer) OnPlatformChannelBind(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	channels := r.Form["channels[]"]

	var err error
	id, _ := strconv.Atoi(idStr)
	_, err = dao.Platform.QueryPlatformByID(id)
	if err == nil {
		err = dao.Platform.BindChannels(id, channels)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, err.Error())
	} else {
		_ = common.HttpResponseJson(w, "OK")
	}
}

func (api *ApiServer) OnPlatformChannelUnbind(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	channels := r.Form["channels[]"]

	var err error
	id, _ := strconv.Atoi(idStr)
	_, err = dao.Platform.QueryPlatformByID(id)
	if err == nil {
		err = dao.Platform.UnbindChannels(id, channels)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, err.Error())
	} else {
		_ = common.HttpResponseJson(w, "OK")
	}
}

func (api *ApiServer) OnDeviceMediaTransportSet(req *SetMediaTransportReq, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	var setupType common.SetupType
	if "udp" == strings.ToLower(req.MediaTransport) {
		setupType = common.SetupTypeUDP
	} else if "passive" == strings.ToLower(req.MediaTransportMode) {
		setupType = common.SetupTypePassive
	} else if "active" == strings.ToLower(req.MediaTransportMode) {
		setupType = common.SetupTypeActive
	} else {
		return nil, fmt.Errorf("media_transport_mode error")
	}

	err := dao.Device.UpdateMediaTransport(req.DeviceID, setupType)
	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCatalogQuery(params *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	deviceModel, err := dao.Device.QueryDevice(params.DeviceID)
	if err != nil {
		return nil, err
	}

	if deviceModel == nil {
		return nil, fmt.Errorf("not found device")
	}

	list, err := (&stack.Device{deviceModel}).QueryCatalog(15)
	if err != nil {
		return nil, err
	}

	response := struct {
		ChannelCount int                 `json:"ChannelCount"`
		ChannelList  []*dao.ChannelModel `json:"ChannelList"`
	}{
		ChannelCount: len(list),
		ChannelList:  list,
	}
	return &response, nil
}

func (api *ApiServer) OnStreamInfo(w http.ResponseWriter, r *http.Request) {
	response, err := stack.MSQueryStreamInfo(r.Header, r.URL.RawQuery)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	defer response.Body.Close()

	// 复制响应头
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// 设置状态码并转发响应体
	w.WriteHeader(response.StatusCode)
	_, err = io.Copy(w, response.Body)
	if err != nil {
		log.Sugar.Errorf("Failed to copy response body: %v", err)
	}
}

func (api *ApiServer) OnSessionList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	//filter := q.Filter // playing-正在播放/stream-不包含回放和下载/record-正在回放的流/hevc-h265流/cascade-级联
	var streams []*dao.StreamModel
	var err error
	if "cascade" == q.Filter {
		protocols := []int{stack.TransStreamGBCascaded}
		var ids []string
		ids, _, err = dao.Sink.QueryStreamIds(protocols, (q.Start/q.Limit)+1, q.Limit)
		if len(ids) > 0 {
			streams, err = dao.Stream.QueryStreamsByIds(ids)
		}
	} else if "stream" == q.Filter {
		streams, _, err = dao.Stream.QueryStreams(q.Keyword, (q.Start/q.Limit)+1, q.Limit, "play")
	} else if "record" == q.Filter {
		streams, _, err = dao.Stream.QueryStreams(q.Keyword, (q.Start/q.Limit)+1, q.Limit, "playback")
	} else if "playing" == q.Filter {
		protocols := []int{stack.TransStreamRtmp, stack.TransStreamFlv, stack.TransStreamRtsp, stack.TransStreamHls, stack.TransStreamRtc}
		var ids []string
		ids, _, err = dao.Sink.QueryStreamIds(protocols, (q.Start/q.Limit)+1, q.Limit)
		if len(ids) > 0 {
			streams, err = dao.Stream.QueryStreamsByIds(ids)
		}
	} else {
		streams, _, err = dao.Stream.QueryStreams(q.Keyword, (q.Start/q.Limit)+1, q.Limit, "")
	}

	if err != nil {
		return nil, err
	}

	response := struct {
		SessionCount int
		SessionList  []*StreamInfo
	}{}

	bytes := make([]byte, 4096)
	for _, stream := range streams {
		values := url.Values{}
		values.Set("streamid", string(stream.StreamID))
		resp, err := stack.MSQueryStreamInfo(r.Header, values.Encode())
		if err != nil {
			return nil, err
		}

		var n int
		n, err = resp.Body.Read(bytes)
		resp.Body.Close()
		if n < 1 {
			break
		}

		info := &StreamInfo{}
		err = json.Unmarshal(bytes[:n], info)
		if err != nil {
			return nil, err
		}

		info.ChannelName = stream.Name
		response.SessionList = append(response.SessionList, info)
	}

	response.SessionCount = len(response.SessionList)
	return &response, nil
}

func (api *ApiServer) OnSessionStop(params *StreamIDParams, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	err := stack.MSCloseSource(params.StreamID)
	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnDeviceTree(q *QueryDeviceChannel, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	var response []*LiveGBSDeviceTree

	// 查询所有设备
	if q.DeviceID == "" && q.PCode == "" {
		devices, err := dao.Device.LoadDevices()
		if err != nil {
			return nil, err
		}

		for _, model := range devices {
			count, _ := dao.Channel.QueryChanelCount(model.DeviceID, true)
			deviceCount, _ := dao.Channel.QueryChanelCount(model.DeviceID, false)
			onlineCount, _ := dao.Channel.QueryOnlineChanelCount(model.DeviceID, false)
			response = append(response, &LiveGBSDeviceTree{Code: "", Custom: false, CustomID: "", CustomName: "", ID: model.DeviceID, Latitude: 0, Longitude: 0, Manufacturer: model.Manufacturer, Name: model.Name, OnlineSubCount: onlineCount, Parental: true, PtzType: 0, Serial: model.DeviceID, Status: model.Status.String(), SubCount: count, SubCountDevice: deviceCount})
		}
	} else {
		// 查询设备下的某个目录的所有通道
		if q.PCode == "" {
			q.PCode = q.DeviceID
		}
		channels, _, _ := dao.Channel.QueryChannels(q.DeviceID, q.PCode, -1, 0, "", "", "asc", "", false)
		for _, channel := range channels {
			id := channel.RootID + ":" + channel.DeviceID
			latitude, _ := strconv.ParseFloat(channel.Latitude, 10)
			longitude, _ := strconv.ParseFloat(channel.Longitude, 10)

			var deviceCount int
			var onlineCount int
			if channel.SubCount > 0 {
				deviceCount, _ = dao.Channel.QuerySubChannelCount(channel.RootID, channel.DeviceID, false)
				onlineCount, _ = dao.Channel.QueryOnlineSubChannelCount(channel.RootID, channel.DeviceID, false)
			}

			response = append(response, &LiveGBSDeviceTree{Code: channel.DeviceID, Custom: false, CustomID: "", CustomName: "", ID: id, Latitude: latitude, Longitude: longitude, Manufacturer: channel.Manufacturer, Name: channel.Name, OnlineSubCount: onlineCount, Parental: false, PtzType: 0, Serial: channel.RootID, Status: channel.Status.String(), SubCount: channel.SubCount, SubCountDevice: deviceCount})
		}
	}

	return &response, nil
}

func (api *ApiServer) OnDeviceRemove(q *DeleteDevice, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	var err error
	if q.IP != "" {
		// 删除IP下的所有设备
		err = dao.Device.DeleteDevicesByIP(q.IP)
	} else if q.UA != "" {
		//  删除UA下的所有设备
		err = dao.Device.DeleteDevicesByUA(q.UA)
	} else {
		// 删除单个设备
		err = dao.Device.DeleteDevice(q.DeviceID)
	}

	if err != nil {
		return nil, err
	} else if q.Forbid {
		if q.IP != "" {
			// 拉黑IP
			err = dao.Blacklist.SaveIP(q.IP)
		} else if q.UA != "" {
			// 拉黑UA
			err = dao.Blacklist.SaveUA(q.UA)
		}
	}

	return "OK", nil
}

func (api *ApiServer) OnEnableSet(params *SetEnable, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	model, err := dao.Platform.QueryPlatformByID(params.ID)
	if err != nil {
		return nil, err
	}

	err = dao.Platform.UpdateEnable(params.ID, params.Enable)
	if err != nil {
		return nil, err
	}

	if params.Enable {
		if stack.PlatformManager.Find(model.ServerAddr) != nil {
			return nil, errors.New("device already started")
		}

		platform, err := stack.NewPlatform(&model.SIPUAOptions, common.SipStack)
		if err != nil {
			_ = dao.Platform.UpdateEnable(params.ID, false)
			return nil, err
		}

		stack.PlatformManager.Add(platform.ServerAddr, platform)
		platform.Start()
	} else if client := stack.PlatformManager.Remove(model.ServerAddr); client != nil {
		client.Stop()
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformChannelList(q *QueryCascadeChannelList, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	response := struct {
		ChannelCount int               `json:"ChannelCount"`
		ChannelList  []*CascadeChannel `json:"ChannelList"`

		ChannelRelateCount *int  `json:"ChannelRelateCount,omitempty"`
		ShareAllChannel    *bool `json:"ShareAllChannel,omitempty"`
	}{}

	id, err := strconv.Atoi(q.ID)
	if err != nil {
		return nil, err
	}

	// livegbs前端, 如果开启级联所有通道, 是不允许再只看已选择或取消绑定通道
	platform, err := dao.Platform.QueryPlatformByID(id)
	if err != nil {
		return nil, err
	}

	// 只看已选择
	if q.Related == true {
		list, total, err := dao.Platform.QueryPlatformChannelList(id)
		if err != nil {
			return nil, err
		}

		response.ChannelCount = total
		ChannelList := ChannelModels2LiveGBSChannels(q.Start+1, list, "")
		for _, channel := range ChannelList {
			response.ChannelList = append(response.ChannelList, &CascadeChannel{
				CascadeID:      q.ID,
				LiveGBSChannel: channel,
			})
		}
	} else {
		list, err := api.OnChannelList(&q.QueryDeviceChannel, w, req)
		if err != nil {
			return nil, err
		}

		result := list.(*ChannelListResult)
		response.ChannelCount = result.ChannelCount

		for _, channel := range result.ChannelList {
			var cascadeId string
			if exist, _ := dao.Platform.QueryPlatformChannelExist(id, channel.DeviceID, channel.ID); exist {
				cascadeId = q.ID
			}

			// 判断该通道是否选中
			response.ChannelList = append(response.ChannelList, &CascadeChannel{
				cascadeId, channel,
			})
		}

		response.ChannelRelateCount = new(int)
		response.ShareAllChannel = new(bool)

		// 级联设备通道总数
		if count, err := dao.Platform.QueryPlatformChannelCount(id); err != nil {
			return nil, err
		} else {
			response.ChannelRelateCount = &count
		}

		*response.ShareAllChannel = platform.ShareAll
	}

	return &response, nil
}

func (api *ApiServer) OnShareAllChannel(q *SetEnable, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	var err error
	if q.ShareAllChannel {
		// 删除所有已经绑定的通道, 设置级联所有通道为true
		if err = dao.Platform.DeletePlatformChannels(q.ID); err == nil {
			err = dao.Platform.SetShareAllChannel(q.ID, true)
		}
	} else {
		// 设置级联所有通道为false
		err = dao.Platform.SetShareAllChannel(q.ID, false)
	}

	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCustomChannelSet(q *CustomChannel, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	if len(q.CustomID) != 20 {
		return nil, fmt.Errorf("20位国标ID")
	}

	if err := dao.Channel.UpdateCustomID(q.DeviceID, q.ChannelID, q.CustomID); err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCatalogPush(q *SetEnable, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	return "OK", nil
}
