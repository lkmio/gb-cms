package stack

import (
	"encoding/xml"
	"gb-cms/common"
	"gb-cms/dao"
	"github.com/ghettovoice/gosip/sip"
)

const (
	DefaultDomainName   = "本域"
	DefaultManufacturer = "github.com/lkmio"
	DefaultModel        = "gb-cms"
	DefaultFirmware     = "dev"
)

type GBClient interface {
	SIPUA

	GBDevice

	SetDeviceInfo(name, manufacturer, model, firmware string)

	// OnQueryCatalog 被查询目录
	OnQueryCatalog(sn int, channels []*dao.ChannelModel)

	// OnQueryDeviceInfo 被查询设备信息
	OnQueryDeviceInfo(sn int)

	OnSubscribeCatalog(sn int)
}

type gbClient struct {
	*sipUA
	Device
	deviceInfo *DeviceInfoResponse
}

func (g *gbClient) OnQueryCatalog(sn int, channels []*dao.ChannelModel) {
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

func NewGBClient(params *common.SIPUAOptions, stack common.SipServer) GBClient {
	ua := &sipUA{
		SIPUAOptions: *params,
		ListenAddr:   stack.ListenAddr(),
		stack:        stack,
	}

	// 心跳间隔最低10秒
	if ua.SIPUAOptions.KeepaliveInterval < 10 {
		ua.SIPUAOptions.KeepaliveInterval = 10
	}

	client := &gbClient{ua, Device{&dao.DeviceModel{DeviceID: params.Username}}, &DeviceInfoResponse{BaseResponse: BaseResponse{BaseMessage: BaseMessage{DeviceID: params.Username, CmdType: CmdDeviceInfo}, Result: "OK"}}}
	return client
}
