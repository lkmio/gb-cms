package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/ghettovoice/gosip/sip"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (api *ApiServer) OnStreamStart(v *InviteParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return api.DoStartStream(v, w, r, "stream")
}

func (api *ApiServer) OnPlaybackStart(v *InviteParams, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if v.Download {
		return api.DoStartStream(v, w, r, "download")
	} else {
		return api.DoStartStream(v, w, r, "playback")
	}
}

func (api *ApiServer) DoStartStream(v *InviteParams, w http.ResponseWriter, r *http.Request, action string) (interface{}, error) {
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

	// 录像下载, 转发到streaminfo接口
	if "download" == action {
		if r.URL.RawQuery == "" {
			r.URL.RawQuery = "streamid=" + string(v.streamId)
		} else if r.URL.RawQuery != "" {
			r.URL.RawQuery += "&streamid=" + string(v.streamId)
		}
		common.HttpForwardTo("/api/v1/stream/info", w, r)
		return nil, nil
	}

	var urls map[string]string
	urls = make(map[string]string, 10)
	for _, streamUrl := range stream.Urls {
		var streamName string

		if strings.HasPrefix(streamUrl, "ws") {
			streamName = "WS_FLV"
		} else if strings.HasSuffix(streamUrl, ".flv") {
			streamName = "FLV"
		} else if strings.HasSuffix(streamUrl, ".m3u8") {
			streamName = "HLS"
		} else if strings.HasSuffix(streamUrl, ".rtc") {
			streamName = "WEBRTC"
		} else if strings.HasPrefix(streamUrl, "rtmp") {
			streamName = "RTMP"
		} else if strings.HasPrefix(streamUrl, "rtsp") {
			streamName = "RTSP"
		}

		// 加上登录的token, 播放授权
		streamUrl += "?stream_token=" + v.Token

		// 兼容livegbs前端播放webrtc
		if streamName == "WEBRTC" {
			if strings.HasPrefix(streamUrl, "http") {
				streamUrl = strings.Replace(streamUrl, "http", "webrtc", 1)
			} else if strings.HasPrefix(streamUrl, "https") {
				streamUrl = strings.Replace(streamUrl, "https", "webrtcs", 1)
			}

			streamUrl += "&wf=livegbs"
		}

		urls[streamName] = streamUrl
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
	if speed < 1 {
		speed = 4
	}
	d := &stack.Device{DeviceModel: device}
	stream, err := d.StartStream(inviteType, params.streamId, params.ChannelID, startTimeSeconds, endTimeSeconds, params.Setup, speed, sync)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, stream, nil
}

func (api *ApiServer) OnCloseStream(v *StreamIDParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	stack.CloseStream(v.StreamID, true)
	return "OK", nil
}

func (api *ApiServer) OnCloseLiveStream(v *InviteParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	id := common.GenerateStreamID(common.InviteTypePlay, v.DeviceID, v.ChannelID, "", "")
	stack.CloseStream(id, true)
	return "OK", nil
}

func (api *ApiServer) OnStreamInfo(w http.ResponseWriter, r *http.Request) {
	common.HttpForwardTo("/api/v1/stream/info", w, r)
}

func (api *ApiServer) OnSessionList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	// 分页参数
	if q.Limit < 1 {
		q.Limit = 10
	}

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
		_ = resp.Body.Close()
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

func (api *ApiServer) OnSessionStop(params *StreamIDParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	err := stack.MSCloseSource(string(params.StreamID))
	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnRecordStart(writer http.ResponseWriter, request *http.Request) {
	common.HttpForwardTo("/api/v1/record/start", writer, request)
}

func (api *ApiServer) OnRecordStop(writer http.ResponseWriter, request *http.Request) {
	common.HttpForwardTo("/api/v1/record/stop", writer, request)
}

func (api *ApiServer) OnPlaybackControl(params *StreamIDParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	if "scale" != params.Command || params.Scale <= 0 || params.Scale > 4 {
		return nil, errors.New("scale error")
	}

	stream, err := dao.Stream.QueryStream(params.StreamID)
	if err != nil {
		return nil, err
	} else if stream.Dialog == nil {
		return nil, errors.New("stream not found")
	}

	// 查找設備
	device, err := dao.Device.QueryDevice(stream.DeviceID)
	if err != nil {
		return nil, err
	}

	s := &stack.Device{DeviceModel: device}
	s.ScalePlayback(stream.Dialog, params.Scale)
	err = stack.MSSpeedSet(string(params.StreamID), params.Scale)
	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnSeekPlayback(v *SeekParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("快进回放 %v", *v)

	model, _ := dao.Stream.QueryStream(v.StreamId)
	if model == nil || model.Dialog == nil {
		log.Sugar.Errorf("快进回放失败 stream不存在 %s", v.StreamId)
		return nil, fmt.Errorf("stream不存在")
	}

	stream := &stack.Stream{StreamModel: model}
	seekRequest := stream.CreateRequestFromDialog(sip.INFO)
	seq, _ := seekRequest.CSeq()
	body := fmt.Sprintf(stack.SeekBodyFormat, seq.SeqNo, v.Seconds)
	seekRequest.SetBody(body, true)
	seekRequest.RemoveHeader(stack.RtspMessageType.Name())
	seekRequest.AppendHeader(&stack.RtspMessageType)

	common.SipStack.SendRequest(seekRequest)
	return nil, nil
}
