package main

import (
	"context"
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"
)

type InviteType string

const (
	InviteTypePlay      = InviteType("play")
	InviteTypePlayback  = InviteType("playback")
	InviteTypeDownload  = InviteType("download")
	InviteTypeBroadcast = InviteType("broadcast")
	InviteTypeTalk      = InviteType("talk")
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
		*i = InviteTypePlay
		break
	}
}

func (d *Device) StartStream(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int, sync bool) (*Stream, error) {
	stream := &Stream{
		DeviceID:  streamId.DeviceID(),
		ChannelID: streamId.ChannelID(),
		StreamID:  streamId,
		Protocol:  SourceType28181,
	}

	// 先添加占位置, 防止重复请求
	oldStream, b := StreamDao.SaveStream(stream)
	if !b {
		if oldStream == nil {
			return nil, fmt.Errorf("stream already exists")
		}
		return oldStream, nil
	}

	dialog, urls, err := d.Invite(inviteType, streamId, channelId, startTime, stopTime, setup, speed)
	if err != nil {
		_, _ = StreamDao.DeleteStream(streamId)
		return nil, err
	}

	stream.SetDialog(dialog)

	// 等待流媒体服务发送推流通知
	wait := func() bool {
		waiting := StreamWaiting{}
		_, _ = EarlyDialogs.Add(string(streamId), &waiting)
		defer EarlyDialogs.Remove(string(streamId))

		ok := http.StatusOK == waiting.Receive(10)
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
	_ = StreamDao.UpdateStream(stream)
	return stream, nil
}

func (d *Device) Invite(inviteType InviteType, streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, []string, error) {
	var err error
	var ssrc string

	defer func() {
		// 如果失败, 告知流媒体服务释放国标源
		if err != nil {
			go MSCloseSource(string(streamId))
		}
	}()

	// 告知流媒体服务创建国标源, 返回收流地址信息
	ip, port, urls, ssrc, msErr := MSCreateGBSource(string(streamId), setup, "", string(inviteType))
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
	SipStack.SendRequestWithContext(reqCtx, inviteRequest, gosip.WithResponseHandler(func(res sip.Response, request sip.Request) {
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

			err = SipStack.Send(ackRequest)
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
	} else if dialogRequest == nil {
		// invite 没有收到任何应答
		return nil, nil, fmt.Errorf("invite request timeout")
	} else if "active" == setup {
		// 如果是TCP主动拉流, 还需要将拉流地址告知给流媒体服务
		var answer *sdp.SDP
		answer, err = sdp.Parse(body)
		if err != nil {
			return nil, nil, err
		}

		addr := fmt.Sprintf("%s:%d", answer.Addr, answer.Video.Port)
		if err = MSConnectGBSource(string(streamId), addr); err != nil {
			Sugar.Errorf("设置GB28181连接地址失败 err: %s addr: %s", err.Error(), addr)
			return nil, nil, err
		}
	}

	return dialogRequest, urls, nil
}

func (d *Device) Play(streamId StreamID, channelId, setup string) (sip.Request, []string, error) {
	return d.Invite(InviteTypePlay, streamId, channelId, "", "", setup, 0)
}

func (d *Device) Playback(streamId StreamID, channelId, startTime, stopTime, setup string) (sip.Request, []string, error) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, 0)

}

func (d *Device) Download(streamId StreamID, channelId, startTime, stopTime, setup string, speed int) (sip.Request, []string, error) {
	return d.Invite(InviteTypePlayback, streamId, channelId, startTime, stopTime, setup, speed)
}
