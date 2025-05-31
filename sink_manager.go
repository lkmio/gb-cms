package main

import (
	"github.com/ghettovoice/gosip/sip"
	"net/http"
	"net/url"
)

func AddForwardSink(forwardType int, request sip.Request, user string, sink *Sink, streamId StreamID, gbSdp *GBSDP, inviteType InviteType, attrs ...string) (sip.Response, error) {
	urlParams := make(url.Values)
	if TransStreamGBTalk == forwardType {
		urlParams.Add("forward_type", "broadcast")
	} else if TransStreamGBCascaded == forwardType {
		urlParams.Add("forward_type", "cascaded")
	} else if TransStreamGBGateway == forwardType {
		urlParams.Add("forward_type", "gateway_1078")
	}

	ip, port, sinkID, err := MSAddForwardSink(forwardType, string(streamId), gbSdp.connectionAddr, gbSdp.offerSetup.String(), gbSdp.answerSetup.String(), gbSdp.ssrc, string(inviteType), urlParams)
	if err != nil {
		Sugar.Errorf("处理上级Invite失败,向流媒体服务添加转发Sink失败 err: %s", err.Error())
		if InviteTypePlay != inviteType {
			CloseStream(streamId, true)
		}

		return nil, err
	}

	sink.SinkID = sinkID
	// 创建answer
	answer := BuildSDP(gbSdp.mediaType, user, gbSdp.sdp.Session, ip, port, gbSdp.startTime, gbSdp.stopTime, gbSdp.answerSetup.String(), gbSdp.speed, gbSdp.ssrc, attrs...)
	response := CreateResponseWithStatusCode(request, http.StatusOK)

	// answer添加contact头域
	response.RemoveHeader("Contact")
	response.AppendHeader(GlobalContactAddress.AsContactHeader())
	response.AppendHeader(&SDPMessageType)
	response.SetBody(answer, true)
	setToTag(response)

	sink.SetDialog(CreateDialogRequestFromAnswer(response, true, request.Source()))

	if err = SinkDao.SaveForwardSink(streamId, sink); err != nil {
		Sugar.Errorf("保存sink到数据库失败, stream: %s sink: %s err: %s", streamId, sink.SinkID, err.Error())
	}

	return response, nil
}

func RemoveForwardSink(StreamID StreamID, sinkID string) *Sink {
	sink, _ := SinkDao.DeleteForwardSink(StreamID, sinkID)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithCallId(callId string) *Sink {
	sink, _ := SinkDao.DeleteForwardSinkByCallID(callId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func RemoveForwardSinkWithSinkStreamID(sinkStreamId StreamID) *Sink {
	sink, _ := SinkDao.DeleteForwardSinkBySinkStreamID(sinkStreamId)
	if sink == nil {
		return nil
	}

	releaseSink(sink)
	return sink
}

func releaseSink(sink *Sink) {
	// 减少拉流计数
	//if stream := StreamManager.Find(sink.StreamID); stream != nil {
	//	stream.DecreaseSinkCount()
	//}
}

func closeSink(sink *Sink, bye, ms bool) {
	releaseSink(sink)

	var callId string
	if sink.Dialog != nil {
		callId_, _ := sink.Dialog.CallID()
		callId = callId_.Value()
	}

	platform := PlatformManager.Find(sink.ServerAddr)
	if platform != nil {
		platform.CloseStream(callId, bye, ms)
	} else {
		sink.Close(bye, ms)
	}
}

func CloseStreamSinks(StreamID StreamID, bye, ms bool) []*Sink {
	sinks, _ := SinkDao.DeleteForwardSinksByStreamID(StreamID)
	for _, sink := range sinks {
		closeSink(sink, bye, ms)
	}

	return sinks
}
