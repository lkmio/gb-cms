package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"net/http"
)

const (
	BroadcastFormat = "<?xml version=\"1.0\" encoding=\"GB2312\" ?>\r\n" +
		"<Notify>\r\n" +
		"<CmdType>Broadcast</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<SourceID>%s</SourceID>\r\n" +
		"<TargetID>%s</TargetID>\r\n" +
		"</Notify>\r\n"
)

func (d *Device) DoBroadcast(sourceId, channelId string) error {
	body := fmt.Sprintf(BroadcastFormat, 1, sourceId, channelId)
	request := d.BuildMessageRequest(channelId, body)

	SipStack.SendRequest(request)
	return nil
}

// OnInvite 语音广播
func (d *Device) OnInvite(request sip.Request, user string) sip.Response {
	// 会话是否存在
	streamWaiting := EarlyDialogs.Find(user)
	if streamWaiting == nil {
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// 解析offer
	sink := streamWaiting.data.(*Sink)
	body := request.Body()
	offer, err := ParseGBSDP(body)
	if err != nil {
		Sugar.Infof("广播失败, 解析sdp发生err: %s  sink: %s  sdp: %s", err.Error(), sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	} else if offer.media == nil {
		Sugar.Infof("广播失败, offer中缺少audio字段. sink: %s sdp: %s", sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// http接口中设置的setup优先级高于sdp中的setup
	if offer.answerSetup != sink.SetupType {
		offer.answerSetup = sink.SetupType
	}

	response, err := AddForwardSink(TransStreamGBTalk, request, user, sink, sink.StreamID, offer, InviteTypeBroadcast, "8 PCMA/8000")
	if err != nil {
		Sugar.Errorf("广播失败, 流媒体创建answer发生err: %s  sink: %s ", err.Error(), sink.SinkID)
		streamWaiting.Put(http.StatusInternalServerError)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	streamWaiting.Put(http.StatusOK)
	return response
}
