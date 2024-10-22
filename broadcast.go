package main

import (
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/transport"
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

func (d *Device) DoBroadcast(sourceId, channelId string) error {
	body := fmt.Sprintf(BroadcastFormat, 1, sourceId, channelId)
	request := d.BuildMessageRequest(channelId, body)

	SipUA.SendRequest(request)
	return nil
}

func (d *Device) OnInvite(request sip.Request, user string) sip.Response {
	session := FindBroadcastSessionWithSourceID(user)
	if session == nil {
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	body := request.Body()
	if body == "" {
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	sdp, err := sdp.Parse(body)
	if err != nil {
		Sugar.Infof("解析sdp失败 err:%s sdp:%s", err.Error(), body)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	if sdp.Audio == nil {
		Sugar.Infof("处理sdp失败 缺少audio字段 sdp:%s", body)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var answerSDP string
	isTcp := strings.Contains(sdp.Audio.Proto, "TCP")
	if !isTcp && BroadcastTypeUDP == session.Type {
		var client *transport.UDPClient
		err := TransportManager.AllocPort(false, func(port uint16) error {
			client = &transport.UDPClient{}
			localAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(Config.ListenIP, strconv.Itoa(int(port))))
			remoteAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(sdp.Addr, strconv.Itoa(int(sdp.Audio.Port))))
			return client.Connect(localAddr, remoteAddr)
		})

		if err == nil {
			Sugar.Errorf("创建UDP广播端口失败 err:%s", err.Error())
			return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
		}

		session.RemoteIP = sdp.Addr
		session.RemotePort = int(sdp.Audio.Port)
		session.Transport = client
		session.Transport.SetHandler(session)
		answerSDP = fmt.Sprintf(AnswerFormat, Config.SipId, Config.PublicIP, Config.PublicIP, client.ListenPort(), "RTP/AVP")
	} else {
		server, err := TransportManager.NewTCPServer(Config.ListenIP)
		if err != nil {
			Sugar.Errorf("创建TCP广播端口失败 err:%s", err.Error())
			return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
		}

		go server.Accept()
		session.Transport = server
		session.Transport.SetHandler(session)
		answerSDP = fmt.Sprintf(AnswerFormat, Config.SipId, Config.PublicIP, Config.PublicIP, server.ListenPort(), "TCP/RTP/AVP")
	}

	response := CreateResponseWithStatusCode(request, http.StatusOK)

	setToTag(response)

	session.Successful = true
	session.ByeRequest = d.CreateDialogRequestFromAnswer(response, true)

	id, _ := request.CallID()
	BroadcastManager.AddSessionWithCallId(id.Value(), session)

	response.SetBody(answerSDP, true)
	response.AppendHeader(&SDPMessageType)
	response.AppendHeader(GlobalContactAddress.AsContactHeader())

	session.Answer <- 0
	return response
}
