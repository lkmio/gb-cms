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

type GBPlatform struct {
	*Client
	lock  sync.Mutex
	sinks map[string]StreamID // 保存级联转发的sink, 方便离线的时候关闭sink
}

func (g *GBPlatform) addSink(callId string, stream StreamID) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.sinks[callId] = stream
}

func (g *GBPlatform) removeSink(callId string) StreamID {
	g.lock.Lock()
	defer g.lock.Unlock()
	stream := g.sinks[callId]
	delete(g.sinks, callId)
	return stream
}

// OnBye 被上级挂断
func (g *GBPlatform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

// CloseStream 关闭级联会话
func (g *GBPlatform) CloseStream(callId string, bye, ms bool) {
	_ = g.removeSink(callId)
	sink := RemoveForwardSinkWithCallId(callId)
	if sink == nil {
		Sugar.Errorf("关闭级联转发sink失败, 找不到sink. callid: %s", callId)
		return
	}

	sink.Close(bye, ms)
}

// CloseStreams 关闭所有级联会话
func (g *GBPlatform) CloseStreams(bye, ms bool) {
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
func (g *GBPlatform) OnInvite(request sip.Request, user string) sip.Response {
	Sugar.Infof("收到级联Invite请求 platform: %s channel: %s sdp: %s", g.SeverID, user, request.Body())

	source := request.Source()
	platform := PlatformManager.Find(source)
	utils.Assert(platform != nil)

	deviceId, channel, err := DB.QueryPlatformChannel(g.ServerAddr, user)
	if err != nil {
		Sugar.Errorf("级联转发失败, 查询数据库失败 err: %s platform: %s channel: %s", err.Error(), g.SeverID, user)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	// 查找通道对应的设备
	device := DeviceManager.Find(deviceId)
	if device == nil {
		Sugar.Errorf("级联转发失败, 设备不存在 device: %s channel: %s", device, user)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	parse, ssrc, speed, media, offerSetup, answerSetup, err := ParseGBSDP(request.Body())
	if err != nil {
		Sugar.Errorf("级联转发失败, 解析上级SDP发生错误 err: %s sdp: %s", err.Error(), request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// 解析时间范围
	time := strings.Split(parse.Time, " ")
	if len(time) < 2 {
		Sugar.Errorf("级联转发失败 上级sdp的时间范围格式错误 time: %s sdp: %s", parse.Time, request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var streamId StreamID
	var inviteType InviteType
	inviteType.SessionName2Type(strings.ToLower(parse.Session))
	switch inviteType {
	case InviteTypePlay:
		streamId = GenerateStreamID(InviteTypePlay, channel.ParentID, user, "", "")
		break
	case InviteTypePlayback:
		// 级联下载和回放不限制路数，也不共享流
		streamId = GenerateStreamID(InviteTypePlayback, channel.ParentID, user, time[0], time[1]) + StreamID("."+utils.RandStringBytes(10))
		break
	case InviteTypeDownload:
		streamId = GenerateStreamID(InviteTypeDownload, channel.ParentID, user, time[0], time[1]) + StreamID("."+utils.RandStringBytes(10))
		break
	}

	stream := StreamManager.Find(streamId)
	addr := fmt.Sprintf("%s:%d", parse.Addr, media.Port)
	if stream == nil {
		s := channel.SetupType.String()
		println(s)
		stream, err = device.(*Device).StartStream(inviteType, streamId, user, time[0], time[1], channel.SetupType.String(), 0, true)
		if err != nil {
			Sugar.Errorf("级联转发失败 err: %s stream: %s", err.Error(), streamId)
			return CreateResponseWithStatusCode(request, http.StatusBadRequest)
		}
	}

	ip, port, sinkID, err := CreateAnswer(string(streamId), addr, offerSetup, answerSetup, ssrc, string(inviteType))
	if err != nil {
		Sugar.Errorf("级联转发失败,向流媒体服务添加转发Sink失败 err: %s", err.Error())

		if "play" != parse.Session {
			CloseStream(streamId, true)
		}

		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	// answer添加contact头域
	answer := BuildSDP(user, parse.Session, ip, port, time[0], time[1], answerSetup, speed, ssrc)
	response := CreateResponseWithStatusCode(request, http.StatusOK)
	response.RemoveHeader("Contact")
	response.AppendHeader(GlobalContactAddress.AsContactHeader())
	response.AppendHeader(&SDPMessageType)
	response.SetBody(answer, true)

	setToTag(response)

	AddForwardSink(streamId, &Sink{
		ID:         sinkID,
		Stream:     streamId,
		ServerAddr: g.ServerAddr,
		Protocol:   "gb_cascaded_forward",
		Dialog:     g.CreateDialogRequestFromAnswer(response, true)})

	return response
}

func (g *GBPlatform) Start() {
	Sugar.Infof("启动级联设备, deivce: %s transport: %s addr: %s", g.Username, g.sipClient.Transport, g.sipClient.ServerAddr)
	g.sipClient.Start()
	g.sipClient.SetOnRegisterHandler(g.onlineCB, g.offlineCB)
}

func (g *GBPlatform) Stop() {
	g.sipClient.Stop()
	g.sipClient.SetOnRegisterHandler(nil, nil)

	// 释放所有推流
	g.CloseStreams(true, true)
}

func (g *GBPlatform) Online() {
	Sugar.Infof("级联设备上线 device: %s", g.SeverID)

	if err := DB.UpdatePlatformStatus(g.SeverID, ON); err != nil {
		Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), g.SeverID)
	}
}

func (g *GBPlatform) Offline() {
	Sugar.Infof("级联设备离线 device: %s", g.SeverID)

	if err := DB.UpdatePlatformStatus(g.SeverID, OFF); err != nil {
		Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), g.SeverID)
	}

	// 释放所有推流
	g.CloseStreams(true, true)
}

func NewGBPlatform(record *SIPUAParams, ua SipServer) (*GBPlatform, error) {
	if len(record.SeverID) != 20 {
		return nil, fmt.Errorf("SeverID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(record.ServerAddr); err != nil {
		return nil, err
	}

	gbClient := NewGBClient(record, ua)
	return &GBPlatform{Client: gbClient.(*Client), sinks: make(map[string]StreamID, 8)}, nil
}
