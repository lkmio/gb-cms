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

	// OnSubscribeCatalog 被订阅目录
	OnSubscribeCatalog(request sip.Request, expires int) (sip.Response, error)

	// OnSubscribeAlarm 被订阅报警
	OnSubscribeAlarm(request sip.Request, expires int) (sip.Response, error)

	// OnSubscribePosition 被订阅位置
	OnSubscribePosition(req sip.Request, expires int) (sip.Response, error)

	CreateRequestByDialogType(t int, method sip.RequestMethod) (sip.Request, error)

	SendMessage(body interface{})

	BuildRequest(method sip.RequestMethod, contentType *sip.ContentType, body string) (sip.Request, error)

	PushCatalog()

	NotifyCatalog(sn int, channels []*dao.ChannelModel, messageFactory func() sip.Request)
}

type gbClient struct {
	*sipUA
	Device
	deviceInfo *DeviceInfoResponse
}

func (g *gbClient) OnQueryCatalog(sn int, channels []*dao.ChannelModel) {
	g.NotifyCatalog(sn, channels, func() sip.Request {
		request, err := BuildMessageRequest(g.sipUA.Username, g.sipUA.ListenAddr, g.sipUA.ServerID, g.sipUA.ServerAddr, g.sipUA.Transport, "")
		if err != nil {
			panic(err)
		}

		return request
	})
}

func (g *gbClient) NotifyCatalog(sn int, channels []*dao.ChannelModel, messageFactory func() sip.Request) {
	response := CatalogResponse{}
	response.SN = sn
	response.CmdType = CmdCatalog
	response.DeviceID = g.sipUA.Username
	response.SumNum = len(channels)

	if response.SumNum < 1 {
		return
	}

	for i, _ := range channels {
		channel := *channels[i]

		// 向上级推送自定义的通道ID
		if channel.CustomID != nil && *channel.CustomID != "" {
			channel.DeviceID = *channel.CustomID
		}

		// 如果设备离线, 状态设置为OFF
		_, b := OnlineDeviceManager.Find(channel.RootID)
		if b {
			channel.Status = common.ON
		} else {
			channel.Status = common.OFF
		}

		response.DeviceList.Devices = nil
		response.DeviceList.Num = 1 // 一次发一个通道
		response.DeviceList.Devices = append(response.DeviceList.Devices, &channel)

		request := messageFactory()
		if request == nil {
			continue
		}

		xmlBody, err := xml.MarshalIndent(&response, " ", "")
		if err != nil {
			panic(err)
		}

		request.SetBody(string(xmlBody), true)
		common.SetHeader(request, &XmlMessageType)

		g.stack.SendRequest(request)
	}
}

func (g *gbClient) SendMessage(msg interface{}) {
	xmlBody, err := xml.MarshalIndent(msg, " ", "")
	if err != nil {
		panic(err)
	}

	request, err := BuildMessageRequest(g.sipUA.Username, g.sipUA.ListenAddr, g.sipUA.ServerID, g.sipUA.ServerAddr, g.sipUA.Transport, string(xmlBody))
	if err != nil {
		panic(err)
	}

	g.sipUA.stack.SendRequest(request)
}

func (g *gbClient) BuildRequest(method sip.RequestMethod, contentType *sip.ContentType, body string) (sip.Request, error) {
	return BuildRequest(method, g.sipUA.Username, g.sipUA.Username, g.sipUA.ServerID, g.sipUA.ServerAddr, g.sipUA.Transport, contentType, body)
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

func (g *gbClient) OnSubscribeCatalog(request sip.Request, expires int) (sip.Response, error) {
	return nil, nil
}

func (g *gbClient) OnSubscribeAlarm(request sip.Request, expires int) (sip.Response, error) {
	return nil, nil
}

func (g *gbClient) OnSubscribePosition(req sip.Request, expires int) (sip.Response, error) {
	return nil, nil
}

func (g *gbClient) CreateRequestByDialogType(t int, method sip.RequestMethod) (sip.Request, error) {
	return nil, nil
}

func (g *gbClient) PushCatalog() {
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
