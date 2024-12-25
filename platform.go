package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
)

// GBPlatformRecord 国标级联设备信息持久化结构体
type GBPlatformRecord struct {
	Username          string       `json:"username"`            // 用户名
	SeverID           string       `json:"server_id"`           // 上级ID, 必选. 作为主键, 不能重复.
	ServerAddr        string       `json:"server_addr"`         // 上级地址, 必选
	Transport         string       `json:"transport"`           // 上级通信方式, UDP/TCP
	Password          string       `json:"password"`            // 密码
	RegisterExpires   int          `json:"register_expires"`    // 注册有效期
	KeepAliveInterval int          `json:"keep_alive_interval"` // 心跳间隔
	CreateTime        string       `json:"create_time"`         // 入库时间
	Status            OnlineStatus `json:"status"`              // 在线状态
}

type GBPlatform struct {
	*Client
	lock    sync.Mutex
	streams map[string]string // 上级会话的callId关联到实际推流通道的callId
}

func (g *GBPlatform) AddStream(callId string, channelCallId string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.streams[callId] = channelCallId
}

func (g *GBPlatform) removeStream(callId string) string {
	g.lock.Lock()
	defer g.lock.Unlock()
	channelCallId := g.streams[callId]
	delete(g.streams, callId)
	return channelCallId
}

// OnBye 被上级挂断
func (g *GBPlatform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

// CloseStream 关闭级联会话
func (g *GBPlatform) CloseStream(callId string, bye, ms bool) {
	channelCallId := g.removeStream(callId)
	stream := StreamManager.FindWithCallId(channelCallId)
	if stream == nil {
		Sugar.Errorf("关闭级联转发sink失败, 找不到stream. callid: %s", callId)
		return
	}

	sink := stream.RemoveForwardStreamSink(callId)
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

	for k := range g.streams {
		callIds = append(callIds, k)
	}

	g.lock.Unlock()

	for _, id := range callIds {
		g.CloseStream(id, bye, ms)
	}
}

// OnInvite 被上级呼叫
func (g *GBPlatform) OnInvite(request sip.Request, user string) sip.Response {
	Sugar.Infof("收到级联Invite请求 platform: %s channel: %s sdp: %s", g.SeverID, user, request.Body())

	source := request.Source()
	platform := PlatformManager.FindPlatformWithServerAddr(source)
	utils.Assert(platform != nil)

	deviceId, channel, err := DB.QueryPlatformChannel(g.SeverID, user)
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
	case InviteTypeLive:
		streamId = GenerateStreamId(InviteTypeLive, channel.ParentID, user, "", "")
		break
	case InviteTypePlayback:
		// 级联下载和回放不限制路数，也不共享流
		streamId = GenerateStreamId(InviteTypePlayback, channel.ParentID, user, time[0], time[1]) + StreamID("."+utils.RandStringBytes(10))
		break
	case InviteTypeDownload:
		streamId = GenerateStreamId(InviteTypeDownload, channel.ParentID, user, time[0], time[1]) + StreamID("."+utils.RandStringBytes(10))
		break
	}

	stream := StreamManager.Find(streamId)
	addr := fmt.Sprintf("%s:%d", parse.Addr, media.Port)
	if stream == nil {
		stream, err = device.(*Device).StartStream(inviteType, streamId, user, time[0], time[1], offerSetup, 0, true)
		if err != nil {
			Sugar.Errorf("级联转发失败 err: %s stream: %s", err.Error(), streamId)
			return CreateResponseWithStatusCode(request, http.StatusBadRequest)
		}
	}

	ssrcInt, _ := strconv.Atoi(ssrc)
	ip, port, sinkID, err := AddForwardStreamSink(string(streamId), addr, offerSetup, uint32(ssrcInt))
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

	// 添加级联转发流
	callID, _ := request.CallID()
	stream.AddForwardStreamSink(callID.Value(), &Sink{
		ID:       sinkID,
		Stream:   streamId,
		ServerID: g.SeverID,
		Dialog:   g.CreateDialogRequestFromAnswer(response, true)},
	)

	return response
}

func (g *GBPlatform) Start() {
	Sugar.Infof("启动级联设备, deivce: %s transport: %s addr: %s", g.SeverID, g.sipClient.Transport, g.sipClient.Domain)
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

func NewGBPlatform(record *GBPlatformRecord, ua SipServer) (*GBPlatform, error) {
	if len(record.SeverID) != 20 {
		return nil, fmt.Errorf("SeverID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(record.ServerAddr); err != nil {
		return nil, err
	}

	gbClient := NewGBClient(record.Username, record.SeverID, record.ServerAddr, record.Transport, record.Password, record.RegisterExpires, record.KeepAliveInterval, ua)
	return &GBPlatform{Client: gbClient.(*Client), streams: make(map[string]string, 8)}, nil
}
