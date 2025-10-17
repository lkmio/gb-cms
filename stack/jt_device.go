package stack

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
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
	log.Sugar.Infof("收到1078的Invite请求 sim number: %s device: %s channel: %s", g.simNumber, g.username, user)

	// 通知1078的信令服务器
	channels, _ := dao.Channel.QueryChannelsByChannelID(user)
	if len(channels) < 1 {
		log.Sugar.Errorf("处理1078的invite失败. 通道不存在 channel: %s device: %s", user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	} else if channels[0].RootID != g.username {
		log.Sugar.Errorf("处理1078的invite失败. 设备和通道不匹配 channel: %s device: %s", user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	channel := channels[0]
	gbsdp, err := ParseGBSDP(request.Body())
	if err != nil {
		log.Sugar.Errorf("处理上级Invite失败, 解析上级SDP发生错误 err: %s sdp: %s", err.Error(), request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var inviteType common.InviteType
	inviteType.SessionName2Type(strings.ToLower(gbsdp.SDP.Session))
	if common.InviteTypePlay != inviteType {
		log.Sugar.Warnf("处理上级Invite失败, 1078暂不支持非实时预览流 inviteType: %s channel: %s device: %s", inviteType, user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusNotImplemented)
	}

	streamId := common.GenerateStreamID(inviteType, g.simNumber, strconv.Itoa(channel.ChannelNumber), gbsdp.StartTime, gbsdp.StopTime)

	sink := &dao.SinkModel{
		StreamID:   streamId,
		ServerAddr: g.ServerAddr,
		Protocol:   TransStreamGBGateway}

	response, err := AddForwardSink(TransStreamGBGateway, request, user, &Sink{sink}, streamId, gbsdp, inviteType, "96 PS/90000")
	if err != nil {
		log.Sugar.Errorf("处理1078的invite失败. 发送hook失败 err: %s channel: %s device: %s", err.Error(), user, g.Username)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	return response
}

func (g *JTDevice) Start() {
	log.Sugar.Infof("启动部标设备, deivce: %s transport: %s addr: %s", g.Username, g.sipUA.Transport, g.sipUA.ServerAddr)
	g.sipUA.Start()
	g.sipUA.SetOnRegisterHandler(g.OnlineCB, g.OfflineCB)
}

func (g *JTDevice) OnlineCB() {
	log.Sugar.Infof("部标设备上线 sim number: %s device: %s server addr: %s", g.simNumber, g.Username, g.ServerAddr)

	if err := dao.JTDevice.UpdateOnlineStatus(common.ON, g.Username); err != nil {
		log.Sugar.Infof("更新部标设备状态失败 err: %s", err.Error())
	}
}

func (g *JTDevice) OfflineCB() {
	log.Sugar.Infof("部标设备离线 sim number: %s device: %s server addr: %s", g.simNumber, g.Username, g.ServerAddr)

	if err := dao.JTDevice.UpdateOnlineStatus(common.OFF, g.Username); err != nil {
		log.Sugar.Infof("更新部标设备状态失败 err: %s", err.Error())
	}

	// 释放所有推流
	g.CloseStreams(true, true)
}

func NewJTDevice(model *dao.JTDeviceModel, ua common.SipServer) (*JTDevice, error) {
	platform, err := NewPlatform(&common.SIPUAOptions{
		Name:              model.Name,
		Username:          model.Username,
		ServerID:          model.SeverID,
		ServerAddr:        model.ServerAddr,
		Transport:         model.Transport,
		Password:          model.Password,
		RegisterExpires:   model.RegisterExpires,
		KeepaliveInterval: model.KeepaliveInterval,
		Status:            model.Status,
	}, ua)
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
