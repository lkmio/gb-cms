package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

const (
	UATypeGB = iota + 1
	UATypeJT
)

type Platform struct {
	*gbClient
	lock  sync.Mutex
	sinks map[string]StreamID // 保存级联转发的sink, 方便离线的时候关闭sink
}

func (g *Platform) addSink(callId string, stream StreamID) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.sinks[callId] = stream
}

func (g *Platform) removeSink(callId string) StreamID {
	g.lock.Lock()
	defer g.lock.Unlock()
	stream := g.sinks[callId]
	delete(g.sinks, callId)
	return stream
}

// OnBye 被上级挂断
func (g *Platform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

// CloseStream 关闭级联会话
func (g *Platform) CloseStream(callId string, bye, ms bool) {
	_ = g.removeSink(callId)
	sink := RemoveForwardSinkWithCallId(callId)
	if sink == nil {
		Sugar.Errorf("关闭转发sink失败, 找不到sink. callid: %s", callId)
		return
	}

	sink.Close(bye, ms)
}

// CloseStreams 关闭所有级联会话
func (g *Platform) CloseStreams(bye, ms bool) {
	var callIds []string
	g.lock.Lock()

	for k := range g.sinks {
		callIds = append(callIds, k)
	}

	g.sinks = make(map[string]StreamID)
	g.lock.Unlock()

	for _, id := range callIds {
		g.CloseStream(id, bye, ms)
	}
}

// OnInvite 被上级呼叫
func (g *Platform) OnInvite(request sip.Request, user string) sip.Response {
	Sugar.Infof("收到上级Invite请求 platform: %s channel: %s sdp: %s", g.SeverID, user, request.Body())

	source := request.Source()
	platform := PlatformManager.Find(source)
	utils.Assert(platform != nil)

	deviceId, channel, err := PlatformDao.QueryPlatformChannel(g.ServerAddr, user)
	if err != nil {
		Sugar.Errorf("处理上级Invite失败, 查询数据库失败 err: %s platform: %s channel: %s", err.Error(), g.SeverID, user)
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
		stream, err = device.StartStream(inviteType, streamId, user, gbSdp.startTime, gbSdp.stopTime, channel.SetupType.String(), 0, true)
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
	Sugar.Infof("ua上线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := PlatformDao.UpdateOnlineStatus(ON, g.ServerAddr); err != nil {
		Sugar.Infof("ua状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}
}

func (g *Platform) Offline() {
	Sugar.Infof("ua离线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := PlatformDao.UpdateOnlineStatus(OFF, g.ServerAddr); err != nil {
		Sugar.Infof("ua状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}

	// 释放所有推流
	g.CloseStreams(true, true)
}

func NewPlatform(record *SIPUAOptions, ua SipServer) (*Platform, error) {
	if len(record.SeverID) != 20 {
		return nil, fmt.Errorf("SeverID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(record.ServerAddr); err != nil {
		return nil, err
	}

	client := NewGBClient(record, ua)
	return &Platform{gbClient: client.(*gbClient), sinks: make(map[string]StreamID, 8)}, nil
}
