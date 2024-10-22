package main

import (
	"context"
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"math"
	"net"
	"strconv"
	"time"
)

type InviteType int

const (
	InviteTypeLive     = InviteType(0)
	InviteTypePlayback = InviteType(1)
	InviteTypeDownload = InviteType(2)
)

func (i *InviteType) SessionName2Type(name string) {
	switch name {
	case "download":
		*i = InviteTypeDownload
		break
	case "playback":
		*i = InviteTypePlayback
		break
	//case "play":
	default:
		*i = InviteTypeLive
		break
	}
}

func (d *Device) StartStream(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int, sync bool) (*Stream, bool) {
	stream := &Stream{
		ID:           streamId,
		forwardSinks: map[string]*Sink{},
	}

	// 先添加占位置, 防止重复请求
	if oldStream, b := StreamManager.Add(stream); !b {
		return oldStream, true
	}

	if dialog, ok := d.Invite(inviteType, streamId, channelId, startTime, stopTime, setup, speed); ok {
		stream.DialogRequest = dialog
		callID, _ := dialog.CallID()
		StreamManager.AddWithCallId(callID.Value(), stream)
	} else {
		StreamManager.Remove(streamId)
		return nil, false
	}

	//开启收流超时
	wait := func() bool {
		ok := stream.WaitForPublishEvent(10)
		if !ok {
			Sugar.Infof("收流超时 发送bye请求...")
			CloseStream(streamId)
		}
		return ok
	}

	if sync {
		go wait()
	} else if !sync && !wait() {
		return nil, false
	}

	return stream, true
}

func (d *Device) Invite(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, bool) {
	var ok bool
	var ssrc string

	defer func() {
		if !ok {
			go CloseGBSource(string(streamId))
		}
	}()

	if InviteTypeLive != inviteType {
		ssrc = GetVodSSRC()
	} else {
		ssrc = GetLiveSSRC()
	}

	ssrcValue, _ := strconv.Atoi(ssrc)
	ip, port, err := CreateGBSource(string(streamId), setup, uint32(ssrcValue))
	if err != nil {
		Sugar.Errorf("创建GBSource失败 err:%s", err.Error())
		return nil, false
	}

	var inviteRequest sip.Request
	if InviteTypePlayback == inviteType {
		inviteRequest, err = d.BuildPlaybackRequest(channelId, ip, port, startTime, stopTime, setup, ssrc)
	} else if InviteTypeDownload == inviteType {
		speed = int(math.Min(4, float64(speed)))
		inviteRequest, err = d.BuildDownloadRequest(channelId, ip, port, startTime, stopTime, setup, speed, ssrc)
	} else {
		inviteRequest, err = d.BuildLiveRequest(channelId, ip, port, setup, ssrc)
	}

	if err != nil {
		Sugar.Errorf("创建invite失败 err:%s", err.Error())
		return nil, false
	}

	var dialogRequest sip.Request
	var answer string
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	SipUA.SendRequestWithContext(reqCtx, inviteRequest, gosip.WithResponseHandler(func(res sip.Response, request sip.Request) {
		if res.StatusCode() < 200 {

		} else if res.StatusCode() == 200 {
			answer = res.Body()
			ackRequest := sip.NewAckRequest("", inviteRequest, res, "", nil)
			ackRequest.AppendHeader(GlobalContactAddress.AsContactHeader())
			//手动替换ack请求目标地址, answer的contact可能不对.
			recipient := ackRequest.Recipient()
			remoteIP, remotePortStr, _ := net.SplitHostPort(d.RemoteAddr)
			remotePort, _ := strconv.Atoi(remotePortStr)
			sipPort := sip.Port(remotePort)
			recipient.SetHost(remoteIP)
			recipient.SetPort(&sipPort)

			Sugar.Infof("send ack %s", ackRequest.String())

			err := SipUA.Send(ackRequest)
			if err != nil {
				cancel()
				Sugar.Errorf("send ack error %s %s", err.Error(), ackRequest.String())
			} else {
				ok = true
				dialogRequest = d.CreateDialogRequestFromAnswer(res, false)
			}
		} else if res.StatusCode() > 299 {
			Sugar.Errorf("invite应答失败 code:%d", res.StatusCode())
			cancel()
		}
	}))

	if !ok {
		return nil, false
	}

	if "active" == setup {
		parse, err := sdp.Parse(answer)
		ok = err == nil && parse.Video != nil && parse.Video.Port != 0
		if !ok {
			Sugar.Errorf("解析应答sdp失败 err:%v sdp:%s", err, answer)
			return nil, false
		}

		addr := fmt.Sprintf("%s:%d", parse.Addr, parse.Video.Port)
		if err = ConnectGBSource(string(streamId), addr); err != nil {
			ok = false
			Sugar.Errorf("设置GB28181连接地址失败 err:%s addr:%s", err.Error(), addr)
		}
	}

	return dialogRequest, ok
}

func (d *Device) Live(streamId StreamID, channelId, setup string) (sip.Request, bool) {
	return d.Invite(InviteTypeLive, streamId, channelId, "", "", setup, 0)
}

func (d *Device) Playback(streamId StreamID, channelId, startTime, stopTime, setup string) (sip.Request, bool) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, 0)

}

func (d *Device) Download(streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, bool) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, speed)
}
