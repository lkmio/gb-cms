package main

import (
	"encoding/xml"
	"fmt"
	"gb-cms/sdp"
	"github.com/ghettovoice/gosip/sip"
	"strconv"
	"strings"
)

const (
	DefaultDomainName   = "本域"
	DefaultManufacturer = "github/lkmio"
	DefaultModel        = "gb-cms"
	DefaultFirmware     = "dev"
)

type GBClient interface {
	SIPUA

	GBDevice

	SetDeviceInfo(name, manufacturer, model, firmware string)

	// OnQueryCatalog 被查询目录
	OnQueryCatalog(sn int, channels []*Channel)

	// OnQueryDeviceInfo 被查询设备信息
	OnQueryDeviceInfo(sn int)

	OnSubscribeCatalog(sn int)
}

type gbClient struct {
	*sipUA
	Device
	deviceInfo *DeviceInfoResponse
}

func (g *gbClient) OnQueryCatalog(sn int, channels []*Channel) {
	response := CatalogResponse{}
	response.SN = sn
	response.CmdType = CmdCatalog
	response.DeviceID = g.sipUA.Username
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
		response.DeviceList.Devices[0].ParentID = g.sipUA.Username

		g.SendMessage(&response)
	}
}

func (g *gbClient) SendMessage(msg interface{}) {
	marshal, err := xml.MarshalIndent(msg, "", " ")
	if err != nil {
		panic(err)
	}

	request, err := BuildMessageRequest(g.sipUA.Username, g.sipUA.ListenAddr, g.sipUA.ServerID, g.sipUA.ServerAddr, g.sipUA.Transport, string(marshal))
	if err != nil {
		panic(err)
	}

	g.sipUA.stack.SendRequest(request)
}

func (g *gbClient) OnQueryDeviceInfo(sn int) {
	g.deviceInfo.SN = sn
	g.SendMessage(&g.deviceInfo)
}

func (g *gbClient) OnInvite(request sip.Request, user string) sip.Response {
	return nil
}

func (g *gbClient) SetDeviceInfo(name, manufacturer, model, firmware string) {
	g.deviceInfo.DeviceName = name
	g.deviceInfo.Manufacturer = manufacturer
	g.deviceInfo.Model = model
	g.deviceInfo.Firmware = firmware
}

func (g *gbClient) OnSubscribeCatalog(sn int) {

}

type GBSDP struct {
	sdp                     *sdp.SDP
	ssrc                    string
	speed                   int
	media                   *sdp.Media
	mediaType               string
	offerSetup, answerSetup SetupType
	startTime, stopTime     string
	connectionAddr          string
	isTcpTransport          bool
}

func ParseGBSDP(body string) (*GBSDP, error) {
	offer, err := sdp.Parse(body)
	if err != nil {
		return nil, err
	}

	gbSdp := &GBSDP{sdp: offer}
	// 解析设置下载速度
	var setup string
	for _, attr := range offer.Attrs {
		if "downloadspeed" == attr[0] {
			speed, err := strconv.Atoi(attr[1])
			if err != nil {
				return nil, err
			}
			gbSdp.speed = speed
		} else if "setup" == attr[0] {
			setup = attr[1]
		}
	}

	// 解析ssrc
	for _, attr := range offer.Other {
		if "y" == attr[0] {
			gbSdp.ssrc = attr[1]
		}
	}

	if offer.Video != nil {
		gbSdp.media = offer.Video
		gbSdp.mediaType = "video"
	} else if offer.Audio != nil {
		gbSdp.media = offer.Audio
		gbSdp.mediaType = "audio"
	}

	tcp := strings.HasPrefix(gbSdp.media.Proto, "TCP")
	if "passive" == setup && tcp {
		gbSdp.offerSetup = SetupTypePassive
		gbSdp.answerSetup = SetupTypeActive
	} else if "active" == setup && tcp {
		gbSdp.offerSetup = SetupTypeActive
		gbSdp.answerSetup = SetupTypePassive
	}

	time := strings.Split(gbSdp.sdp.Time, " ")
	if len(time) < 2 {
		return nil, fmt.Errorf("sdp的时间范围格式错误 time: %s sdp: %s", gbSdp.sdp.Time, body)
	}

	gbSdp.startTime = time[0]
	gbSdp.stopTime = time[1]
	gbSdp.isTcpTransport = tcp
	gbSdp.connectionAddr = fmt.Sprintf("%s:%d", gbSdp.sdp.Addr, gbSdp.media.Port)
	return gbSdp, nil
}

func NewGBClient(params *SIPUAOptions, stack SipServer) GBClient {
	ua := &sipUA{
		SIPUAOptions: *params,
		ListenAddr:   stack.ListenAddr(),
		stack:        stack,
	}

	// 心跳间隔最低10秒
	if ua.SIPUAOptions.KeepaliveInterval < 10 {
		ua.SIPUAOptions.KeepaliveInterval = 10
	}

	client := &gbClient{ua, Device{DeviceID: params.Username}, &DeviceInfoResponse{BaseResponse: BaseResponse{BaseMessage: BaseMessage{DeviceID: params.Username, CmdType: CmdDeviceInfo}, Result: "OK"}}}
	return client
}
