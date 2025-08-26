package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/log"
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

	common.SipStack.SendRequest(request)
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
	sink := streamWaiting.Data.(*Sink)
	body := request.Body()
	offer, err := ParseGBSDP(body)
	if err != nil {
		log.Sugar.Infof("广播失败, 解析sdp发生err: %s  sink: %s  sdp: %s", err.Error(), sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	} else if offer.Media == nil {
		log.Sugar.Infof("广播失败, offer中缺少audio字段. sink: %s sdp: %s", sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// http接口中设置的setup优先级高于sdp中的setup
	if offer.AnswerSetup != sink.SetupType {
		offer.AnswerSetup = sink.SetupType
	}

	response, err := AddForwardSink(TransStreamGBTalk, request, user, sink, sink.StreamID, offer, common.InviteTypeBroadcast, "8 PCMA/8000")
	if err != nil {
		log.Sugar.Errorf("广播失败, 流媒体创建answer发生err: %s  sink: %s ", err.Error(), sink.SinkID)
		streamWaiting.Put(http.StatusInternalServerError)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	streamWaiting.Put(http.StatusOK)
	return response
}
