package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// GBPlatformRecord 国标上级信息持久化结构体
type GBPlatformRecord struct {
	Username          string `json:"username"`            //用户名
	SeverID           string `json:"server_id"`           //上级ID, 必选
	ServerAddr        string `json:"server_addr"`         //上级地址, 必选
	Transport         string `json:"transport"`           //上级通信方式, UDP/TCP
	Password          string `json:"password"`            //密码
	RegisterExpires   int    `json:"register_expires"`    //注册有效期
	KeepAliveInterval int    `json:"keep_alive_interval"` //心跳间隔
}

type GBPlatform struct {
	*Client
	streams *streamManager // 保存与上级的所有级联会话
}

// OnBye 被上级挂断
func (g *GBPlatform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

// CloseStream 关闭级联会话
func (g *GBPlatform) CloseStream(id string, bye, ms bool) {
	// 删除会话
	stream := g.streams.RemoveWithCallId(id)
	if stream == nil {
		return
	}

	// 从国标源中删除当前转发流
	sink := stream.RemoveForwardSink(id)

	if ms {
		// 通知媒体服务
		go CloseSink(string(stream.ID), sink.id)
	}

	// SIP挂断
	if bye {
		byeRequest := CreateRequestFromDialog(sink.dialog, sip.BYE)
		SipUA.SendRequest(byeRequest)
	}
}

// OnInvite 被上级呼叫
func (g *GBPlatform) OnInvite(request sip.Request, user string) sip.Response {
	Sugar.Infof("接收到上级预览 上级id: %s 请求通道id: %s sdp: %s", g.SeverId, user, request.Body())

	channel := g.FindChannel(user)

	// 查找通道对应的设备
	device := DeviceManager.Find(channel.ParentID)
	if device == nil {
		Sugar.Errorf("级联转发失败 设备不存在 DeviceID: %s ChannelID: %s", channel.DeviceID, user)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	parse, ssrc, speed, media, offerSetup, answerSetup, err := ParseGBSDP(request.Body())
	if err != nil {
		Sugar.Errorf("级联转发失败 解析上级SDP发生错误 err: %s sdp: %s", err.Error(), request.Body())
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
		Sugar.Errorf("级联转发失败 向流媒体服务添加转发Sink失败 err: %s", err.Error())

		if "play" != parse.Session {
			CloseStream(streamId)
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
	stream.AddForwardSink(callID.Value(), &Sink{sinkID, g.ID, g.CreateDialogRequestFromAnswer(response, true), g.Username})

	// 保存与上级的会话
	g.streams.AddWithCallId(callID.Value(), stream)
	return response
}

func NewGBPlatform(record *GBPlatformRecord, ua SipServer) (*GBPlatform, error) {
	if len(record.SeverID) != 20 {
		return nil, fmt.Errorf("SeverID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(record.ServerAddr); err != nil {
		return nil, err
	}

	client := NewGBClient(record.Username, record.SeverID, record.ServerAddr, record.Transport, record.Password, record.RegisterExpires, record.KeepAliveInterval, ua)
	return &GBPlatform{client.(*Client), NewStreamManager()}, nil
}
