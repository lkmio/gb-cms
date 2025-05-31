package main

import (
	"github.com/ghettovoice/gosip/sip"
	"net/http"
	"strconv"
	"strings"
)

type JTDevice struct {
	*Platform
	username  string
	simNumber string
}

func (g *JTDevice) OnInvite(request sip.Request, user string) sip.Response {
	// 通知1078的信令服务器
	channels, _ := ChannelDao.QueryChannelsByChannelID(user)
	if len(channels) < 1 {
		Sugar.Errorf("处理1078的invite失败. 通道不存在 channel: %s device: %s", user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	} else if channels[0].RootID != g.username {
		Sugar.Errorf("处理1078的invite失败. 设备和通道不匹配 channel: %s device: %s", user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	channel := channels[0]
	gbsdp, err := ParseGBSDP(request.Body())
	if err != nil {
		Sugar.Errorf("处理上级Invite失败, 解析上级SDP发生错误 err: %s sdp: %s", err.Error(), request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var inviteType InviteType
	inviteType.SessionName2Type(strings.ToLower(gbsdp.sdp.Session))
	if InviteTypePlay != inviteType {
		Sugar.Warnf("处理上级Invite失败, 1078暂不支持非实时预览流 inviteType: %s channel: %s device: %s", inviteType, user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotImplemented)
	}

	streamId := GenerateStreamID(inviteType, g.simNumber, strconv.Itoa(channel.ChannelNumber), gbsdp.startTime, gbsdp.stopTime)

	sink := &Sink{
		StreamID:   streamId,
		ServerAddr: g.ServerAddr,
		Protocol:   "gb_gateway"}

	response, err := AddForwardSink(TransStreamGBGateway, request, user, sink, streamId, gbsdp, inviteType, "96 PS/90000")
	if err != nil {
		Sugar.Errorf("处理1078的invite失败. 发送hook失败 err: %s channel: %s device: %s", err.Error(), user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	return response
}

func NewJTDevice(model *JTDeviceModel, ua SipServer) (*JTDevice, error) {
	platform, err := NewPlatform(&model.SIPUAOptions, ua)
	if err != nil {
		return nil, err
	}

	platform.SetDeviceInfo(model.Name, model.Manufacturer, model.Model, model.Firmware)

	return &JTDevice{
		Platform:  platform,
		username:  model.Username,
		simNumber: model.SimNumber,
	}, nil
}
