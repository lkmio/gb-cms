package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"net/netip"
	"strings"
)

const (
	UATypeGB = iota + 1
	UATypeJT
)

type Platform struct {
	*gbClient
}

// OnBye 被上级挂断
func (g *Platform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

func (g *Platform) OnQueryCatalog(sn int, channels []*Channel) {
	// 添加本级域
	channels = append(channels, &Channel{
		DeviceID: g.ServerID,
		Setup:    SetupTypePassive,
	})

	g.gbClient.OnQueryCatalog(sn, channels)
}

// CloseStream 关闭级联会话
func (g *Platform) CloseStream(callId string, bye, ms bool) {
	sink, _ := SinkDao.DeleteForwardSinkByCallID(callId)
	if sink != nil {
		sink.Close(bye, ms)
	}
}

// CloseStreams 关闭所有级联会话
func (g *Platform) CloseStreams(bye, ms bool) {
	sinks, _ := SinkDao.DeleteForwardSinksByServerAddr(g.ServerAddr)
	for _, sink := range sinks {
		sink.Close(bye, ms)
	}
}

// OnInvite 被上级呼叫
func (g *Platform) OnInvite(request sip.Request, user string) sip.Response {
	Sugar.Infof("收到上级Invite请求 platform: %s channel: %s sdp: %s", g.ServerID, user, request.Body())

	source := request.Source()
	platform := PlatformManager.Find(source)
	utils.Assert(platform != nil)

	deviceId, channel, err := PlatformDao.QueryPlatformChannel(g.ServerAddr, user)
	if err != nil {
		Sugar.Errorf("处理上级Invite失败, 查询数据库失败 err: %s platform: %s channel: %s", err.Error(), g.ServerID, user)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	// 查找通道对应的设备
	device, _ := DeviceDao.QueryDevice(deviceId)
	if device == nil {
		Sugar.Errorf("处理上级Invite失败, 设备不存在 device: %s channel: %s", device, user)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	gbSdp, err := ParseGBSDP(request.Body())
	if err != nil {
		Sugar.Errorf("处理上级Invite失败,err: %s sdp: %s", err.Error(), request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var inviteType InviteType
	inviteType.SessionName2Type(strings.ToLower(gbSdp.sdp.Session))
	streamId := GenerateStreamID(inviteType, channel.RootID, channel.DeviceID, gbSdp.startTime, gbSdp.stopTime)

	// 如果流不存在, 向通道发送Invite请求
	stream, _ := StreamDao.QueryStream(streamId)
	if stream == nil {
		stream, err = device.StartStream(inviteType, streamId, user, gbSdp.startTime, gbSdp.stopTime, channel.Setup.String(), 0, true)
		if err != nil {
			Sugar.Errorf("处理上级Invite失败 err: %s stream: %s", err.Error(), streamId)
			return CreateResponseWithStatusCode(request, http.StatusBadRequest)
		}
	}

	sink := &Sink{
		StreamID:   streamId,
		ServerAddr: g.ServerAddr,
		Protocol:   "gb_cascaded"}

	response, err := AddForwardSink(TransStreamGBCascaded, request, user, sink, streamId, gbSdp, inviteType, "96 PS/90000")
	if err != nil {
		Sugar.Errorf("处理上级Invite失败 err: %s stream: %s", err.Error(), streamId)
	}

	return response
}

func (g *Platform) Start() {
	Sugar.Infof("启动级联设备, deivce: %s transport: %s addr: %s", g.Username, g.sipUA.Transport, g.sipUA.ServerAddr)
	g.sipUA.Start()
	g.sipUA.SetOnRegisterHandler(g.Online, g.Offline)
}

func (g *Platform) Stop() {
	g.sipUA.Stop()
	g.sipUA.SetOnRegisterHandler(nil, nil)

	// 释放所有推流
	g.CloseStreams(true, true)
}

func (g *Platform) Online() {
	Sugar.Infof("级联设备上线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := PlatformDao.UpdateOnlineStatus(ON, g.ServerAddr); err != nil {
		Sugar.Infof("更新级联设备状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}
}

func (g *Platform) Offline() {
	Sugar.Infof("级联设备离线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := PlatformDao.UpdateOnlineStatus(OFF, g.ServerAddr); err != nil {
		Sugar.Infof("更新级联设备状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}

	// 释放所有推流
	g.CloseStreams(true, true)
}

func NewPlatform(options *SIPUAOptions, ua SipServer) (*Platform, error) {
	if len(options.ServerID) != 20 {
		return nil, fmt.Errorf("ServerID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(options.ServerAddr); err != nil {
		return nil, err
	}

	client := NewGBClient(options, ua)
	return &Platform{gbClient: client.(*gbClient)}, nil
}
