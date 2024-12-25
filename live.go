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

func (d *Device) StartStream(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int, sync bool) (*Stream, error) {
	stream := &Stream{
		ID:                 streamId,
		ForwardStreamSinks: map[string]*Sink{},
		CreateTime:         time.Now().UnixMilli(),
	}

	// 先添加占位置, 防止重复请求
	if oldStream, b := StreamManager.Add(stream); !b {
		return oldStream, nil
	}

	dialog, urls, err := d.Invite(inviteType, streamId, channelId, startTime, stopTime, setup, speed)
	if err != nil {
		StreamManager.Remove(streamId)
		return nil, err
	}

	stream.Dialog = dialog
	callID, _ := dialog.CallID()
	StreamManager.AddWithCallId(callID.Value(), stream)

	// 等待流媒体服务发送推流通知
	wait := func() bool {
		ok := stream.WaitForPublishEvent(10)
		if !ok {
			Sugar.Infof("收流超时 发送bye请求...")
			CloseStream(streamId, true)
		}
		return ok
	}

	if sync {
		go wait()
	} else if !sync && !wait() {
		return nil, fmt.Errorf("receiving stream timed out")
	}

	stream.urls = urls

	// 保存到数据库
	go DB.SaveStream(stream)

	return stream, nil
}

func (d *Device) Invite(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, []string, error) {
	var err error
	var ssrc string

	defer func() {
		// 如果失败, 告知流媒体服务释放国标源
		if err != nil {
			go CloseSource(string(streamId))
		}
	}()

	// 生成下发的ssrc
	if InviteTypeLive != inviteType {
		ssrc = GetVodSSRC()
	} else {
		ssrc = GetLiveSSRC()
	}

	// 告知流媒体服务创建国标源, 返回收流地址信息
	ssrcValue, _ := strconv.Atoi(ssrc)
	ip, port, urls, msErr := CreateGBSource(string(streamId), setup, uint32(ssrcValue))
	if msErr != nil {
		Sugar.Errorf("创建GBSource失败 err: %s", msErr.Error())
		return nil, nil, msErr
	}

	// 创建invite请求
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
		Sugar.Errorf("创建invite失败 err: %s", err.Error())
		return nil, nil, err
	}

	var dialogRequest sip.Request
	var body string
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// invite信令交互
	SipUA.SendRequestWithContext(reqCtx, inviteRequest, gosip.WithResponseHandler(func(res sip.Response, request sip.Request) {
		if res.StatusCode() < 200 {

		} else if res.StatusCode() == 200 {
			body = res.Body()
			ackRequest := sip.NewAckRequest("", inviteRequest, res, "", nil)
			ackRequest.AppendHeader(GlobalContactAddress.AsContactHeader())

			// 手动替换ack请求目标地址, answer的contact可能不对.
			recipient := ackRequest.Recipient()
			remoteIP, remotePortStr, _ := net.SplitHostPort(d.RemoteAddr)
			remotePort, _ := strconv.Atoi(remotePortStr)
			sipPort := sip.Port(remotePort)
			recipient.SetHost(remoteIP)
			recipient.SetPort(&sipPort)

			Sugar.Infof("send ack %s", ackRequest.String())

			err = SipUA.Send(ackRequest)
			if err != nil {
				cancel()
				Sugar.Errorf("send ack error %s %s", err.Error(), ackRequest.String())
			} else {
				dialogRequest = d.CreateDialogRequestFromAnswer(res, false)
			}
		} else if res.StatusCode() > 299 {
			err = fmt.Errorf("answer has a bad status code: %d", res.StatusCode())
			Sugar.Errorf("%s response: %s", err.Error(), res.String())
			cancel()
		}
	}))

	if err != nil {
		return nil, nil, err
	} else if "active" == setup {
		// 如果是TCP主动拉流, 还需要将拉流地址告知给流媒体服务
		var answer *sdp.SDP
		answer, err = sdp.Parse(body)
		if err != nil {
			return nil, nil, err
		}

		addr := fmt.Sprintf("%s:%d", answer.Addr, answer.Video.Port)
		if err = ConnectGBSource(string(streamId), addr); err != nil {
			Sugar.Errorf("设置GB28181连接地址失败 err: %s addr: %s", err.Error(), addr)
			return nil, nil, err
		}
	}

	return dialogRequest, urls, nil
}

func (d *Device) Live(streamId StreamID, channelId, setup string) (sip.Request, []string, error) {
	return d.Invite(InviteTypeLive, streamId, channelId, "", "", setup, 0)
}

func (d *Device) Playback(streamId StreamID, channelId, startTime, stopTime, setup string) (sip.Request, []string, error) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, 0)

}

func (d *Device) Download(streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, []string, error) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, speed)
}
