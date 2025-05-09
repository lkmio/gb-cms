package main

import (
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const (
	BroadcastFormat = "<?xml version=\"1.0\" encoding=\"GB2312\" ?>\r\n" +
		"<Notify>\r\n" +
		"<CmdType>Broadcast</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<SourceID>%s</SourceID>\r\n" +
		"<TargetID>%s</TargetID>\r\n" +
		"</Notify>\r\n"

	AnswerFormat = "v=0\r\n" +
		"o=%s 0 0 IN IP4 %s\r\n" +
		"s=Play\r\n" +
		"c=IN IP4 %s\r\n" +
		"t=0 0\r\n" +
		"m=audio %d %s 8\r\n" +
		"a=sendonly\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n"
)

func findSetup(descriptor *sdp.SDP) SetupType {
	var tcp bool
	if descriptor.Audio != nil {
		tcp = strings.Contains(descriptor.Audio.Proto, "TCP")
	}

	if !tcp && descriptor.Video != nil {
		tcp = strings.Contains(descriptor.Video.Proto, "TCP")
	}

	setup := SetupTypeUDP
	if tcp {
		for _, attr := range descriptor.Attrs {
			if "setup" == attr[0] {
				if SetupTypePassive.String() == attr[1] {
					setup = SetupTypePassive
				} else if SetupTypeActive.String() == attr[1] {
					setup = SetupTypeActive
				}
			}
		}
	}

	return setup
}

func (d *Device) DoBroadcast(sourceId, channelId string) error {
	body := fmt.Sprintf(BroadcastFormat, 1, sourceId, channelId)
	request := d.BuildMessageRequest(channelId, body)

	SipUA.SendRequest(request)
	return nil
}

// OnInvite 语音广播
func (d *Device) OnInvite(request sip.Request, user string) sip.Response {
	sink := BroadcastDialogs.Find(user)
	if sink == nil {
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	body := request.Body()
	offer, err := sdp.Parse(body)
	if err != nil {
		Sugar.Infof("广播失败, 解析sdp发生err: %s  sink: %s  sdp: %s", err.Error(), sink.ID, body)
		sink.onPublishCb <- http.StatusBadRequest
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	} else if offer.Audio == nil {
		Sugar.Infof("广播失败, offer中缺少audio字段. sink: %s sdp: %s", sink.ID, body)
		sink.onPublishCb <- http.StatusBadRequest
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// 通知流媒体服务器创建answer
	offerSetup := findSetup(offer)
	answerSetup := sink.SetupType
	finalSetup := offerSetup
	if answerSetup != offerSetup {
		finalSetup = answerSetup
	}

	addr := net.JoinHostPort(offer.Addr, strconv.Itoa(int(offer.Audio.Port)))
	host, port, sinkId, err := CreateAnswer(string(sink.Stream), addr, offerSetup.String(), answerSetup.String(), "", string(InviteTypeBroadcast))
	if err != nil {
		Sugar.Errorf("广播失败, 流媒体创建answer发生err: %s  sink: %s ", err.Error(), sink.ID)
		sink.onPublishCb <- http.StatusInternalServerError
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	var answerSDP string
	// UDP广播
	if SetupTypeUDP == finalSetup {
		answerSDP = fmt.Sprintf(AnswerFormat, Config.SipId, host, host, port, "RTP/AVP")
	} else {
		// TCP广播
		answerSDP = fmt.Sprintf(AnswerFormat, Config.SipId, host, host, port, "TCP/RTP/AVP")
	}

	// 创建answer和dialog
	response := CreateResponseWithStatusCode(request, http.StatusOK)
	setToTag(response)

	sink.ID = sinkId
	sink.Dialog = d.CreateDialogRequestFromAnswer(response, true)

	response.SetBody(answerSDP, true)
	response.AppendHeader(&SDPMessageType)
	response.AppendHeader(GlobalContactAddress.AsContactHeader())

	sink.onPublishCb <- http.StatusOK
	return response
}
