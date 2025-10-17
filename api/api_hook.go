package api

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/hook"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"strings"
)

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
				_ = dao.Sink.CreateSink(&dao.SinkModel{
					SinkID:     params.Sink,
					StreamID:   params.Stream,
					Protocol:   params.Protocol,
					RemoteAddr: params.RemoteAddr,
				})
			}
			return
		} else if stack.TransStreamGBTalk == params.Protocol {
			// 对讲/广播
			w.WriteHeader(http.StatusOK)
			return
		}

		// 级联, 在此处请求流
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

			_ = dao.Sink.CreateSink(&dao.SinkModel{
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

	sink, _ := dao.Sink.DeleteSink(params.Sink)
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

func (api *ApiServer) OnStarted(_ http.ResponseWriter, _ *http.Request) {
	log.Sugar.Infof("lkm启动")

	streams, _ := dao.Stream.DeleteStreams()
	for _, stream := range streams {
		(&stack.Stream{StreamModel: stream}).Close(true, false)
	}

	sinks, _ := dao.Sink.DeleteSinks()
	for _, sink := range sinks {
		(&stack.Sink{SinkModel: sink}).Close(true, false)
	}
}
