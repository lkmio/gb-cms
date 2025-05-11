package main

import (
	"encoding/xml"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip/sip"
	"strconv"
	"strings"
)

type GBClient interface {
	SipClient

	GBDevice

	SetDeviceInfo(name, manufacturer, model, firmware string)

	// OnQueryCatalog 被查询目录
	OnQueryCatalog(sn int, channels []*Channel)

	// OnQueryDeviceInfo 被查询设备信息
	OnQueryDeviceInfo(sn int)

	OnSubscribeCatalog(sn int)
}

type Client struct {
	*sipClient
	Device
	deviceInfo *DeviceInfoResponse
}

func (g *Client) OnQueryCatalog(sn int, channels []*Channel) {
	response := CatalogResponse{}
	response.SN = sn
	response.CmdType = CmdCatalog
	response.DeviceID = g.sipClient.Username
	response.SumNum = len(channels)

	if response.SumNum < 1 {
		g.SendMessage(&response)
		return
	}

	for i, _ := range channels {
		channel := *channels[i]

		response.DeviceList.Devices = nil
		response.DeviceList.Num = 1 // 一次发一个通道
		response.DeviceList.Devices = append(response.DeviceList.Devices, &channel)
		response.DeviceList.Devices[0].ParentID = g.sipClient.Username

		g.SendMessage(&response)
	}
}

func (g *Client) SendMessage(msg interface{}) {
	marshal, err := xml.MarshalIndent(msg, "", " ")
	if err != nil {
		panic(err)
	}

	request, err := BuildMessageRequest(g.sipClient.Username, g.sipClient.ListenAddr, g.sipClient.SeverID, g.sipClient.ServerAddr, g.sipClient.Transport, string(marshal))
	if err != nil {
		panic(err)
	}

	g.sipClient.ua.SendRequest(request)
}

func (g *Client) OnQueryDeviceInfo(sn int) {
	g.deviceInfo.SN = sn
	g.SendMessage(&g.deviceInfo)
}

func (g *Client) OnInvite(request sip.Request, user string) sip.Response {
	return nil
}

func (g *Client) SetDeviceInfo(name, manufacturer, model, firmware string) {
	g.deviceInfo.DeviceName = name
	g.deviceInfo.Manufacturer = manufacturer
	g.deviceInfo.Model = model
	g.deviceInfo.Firmware = firmware
}

func (g *Client) OnSubscribeCatalog(sn int) {

}

func ParseGBSDP(body string) (offer *sdp.SDP, ssrc string, speed int, media *sdp.Media, offerSetup, answerSetup string, err error) {
	offer, err = sdp.Parse(body)
	if err != nil {
		return nil, "", 0, nil, "", "", err
	}

	// 解析设置下载速度
	var setup string
	for _, attr := range offer.Attrs {
		if "downloadspeed" == attr[0] {
			speed, err = strconv.Atoi(attr[1])
			if err != nil {
				return nil, "", 0, nil, "", "", err
			}
		} else if "setup" == attr[0] {
			setup = attr[1]
		}
	}

	// 解析ssrc
	for _, attr := range offer.Other {
		if "y" == attr[0] {
			ssrc = attr[1]
		}
	}

	if offer.Video != nil {
		media = offer.Video
	} else if offer.Audio != nil {
		media = offer.Audio
	}

	tcp := strings.HasPrefix(media.Proto, "TCP")
	if "passive" == setup && tcp {
		offerSetup = "passive"
		answerSetup = "active"
	} else if "active" == setup && tcp {
		offerSetup = "active"
		answerSetup = "passive"
	}

	return
}

func NewGBClient(params *SIPUAParams, ua SipServer) GBClient {
	sip := &sipClient{
		SIPUAParams: *params,
		ListenAddr:  ua.ListenAddr(),
		ua:          ua,
	}

	client := &Client{sip, Device{ID: params.Username}, &DeviceInfoResponse{BaseResponse: BaseResponse{BaseMessage: BaseMessage{DeviceID: params.Username, CmdType: CmdDeviceInfo}, Result: "OK"}}}
	return client
}
