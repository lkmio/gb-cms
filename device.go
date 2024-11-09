package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"strconv"
	"sync"
)

const (
	CatalogFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>Catalog</CmdType>\r\n" +
		"<SN>" +
		"%s" +
		"</SN>\r\n" +
		"<DeviceID>" +
		"%s" +
		"</DeviceID>\r\n" +
		"</Query>\r\n"
)

var (
	XmlMessageType sip.ContentType = "Application/MANSCDP+xml"

	SDPMessageType sip.ContentType = "application/sdp"
)

type GBDevice interface {
	GetID() string

	QueryCatalog()

	QueryRecord(channelId, startTime, endTime string, sn int, type_ string) error

	//Invite(channel string, setup string)

	OnCatalog(response *CatalogResponse)

	OnRecord(response *QueryRecordInfoResponse)

	OnDeviceInfo(response *DeviceInfoResponse)

	// OnInvite 语音广播
	OnInvite(request sip.Request, user string) sip.Response

	// OnBye 设备侧主动挂断
	OnBye(request sip.Request)

	OnNotifyPosition(notify *MobilePositionNotify)

	//
	//OnNotifyCatalog()
	//
	//OnNotifyAlarm()

	SubscribePosition(channelId string) error

	//SubscribeCatalog()
	//
	//SubscribeAlarm()

	Broadcast(sourceId, channelId string) sip.ClientTransaction

	OnKeepalive()

	// AddChannels 批量添加通道
	AddChannels(channels []*Channel)

	// GetChannels 获取所有通道
	GetChannels() []*Channel

	// FindChannel 根据通道ID查找通道
	FindChannel(id string) *Channel

	// RemoveChannel 根据通道ID删除通道
	RemoveChannel(id string) *Channel

	// UpdateChannel 订阅目录，通道发生改变
	// 附录P.4.2.2
	// @Params event ON-上线/OFF-离线/VLOST-视频丢失/DEFECT-故障/ADD-增加/DEL-删除/UPDATE-更新
	UpdateChannel(id string, event string)
}

type Device struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	RemoteAddr string              `json:"remote_addr"`
	Transport  string              `json:"transport"`
	Status     string              `json:"Status,omitempty"` //在线状态 ON-在线/OFF-离线
	Channels   map[string]*Channel `json:"channels"`
	lock       sync.RWMutex
}

func (d *Device) GetID() string {
	return d.ID
}

func (d *Device) BuildMessageRequest(to, body string) sip.Request {
	request, err := BuildMessageRequest(Config.SipId, net.JoinHostPort(GlobalContactAddress.Uri.Host(), GlobalContactAddress.Uri.Port().String()), to, d.RemoteAddr, d.Transport, body)
	if err != nil {
		panic(err)
	}
	return request
}

func (d *Device) QueryCatalog() {
	body := fmt.Sprintf(CatalogFormat, "1", d.ID)
	request := d.BuildMessageRequest(d.ID, body)
	SipUA.SendRequest(request)
}

func (d *Device) QueryRecord(channelId, startTime, endTime string, sn int, type_ string) error {
	body := fmt.Sprintf(QueryRecordFormat, sn, channelId, startTime, endTime, type_)
	request := d.BuildMessageRequest(channelId, body)
	SipUA.SendRequest(request)
	return nil
}

func (d *Device) OnBye(request sip.Request) {

}

func (d *Device) OnCatalog(response *CatalogResponse) {
	for _, device := range response.DeviceList.Devices {
		device.ParentID = d.ID
	}

	d.AddChannels(response.DeviceList.Devices)
}

func (d *Device) OnRecord(response *QueryRecordInfoResponse) {
	event := SNManager.FindEvent(response.SN)
	if event == nil {
		Sugar.Errorf("处理录像查询响应失败 SN:%d", response.SN)
		return
	}

	event(response)
}

func (d *Device) OnDeviceInfo(response *DeviceInfoResponse) {

}

func (d *Device) OnNotifyPosition(notify *MobilePositionNotify) {

}

func (d *Device) SubscribePosition(channelId string) error {
	if channelId == "" {
		channelId = d.ID
	}

	//暂时不考虑级联
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, Config.SipId, Config.SipContactAddr, channelId)
	body := fmt.Sprintf(MobilePositionMessageFormat, "1", channelId, Config.MobilePositionInterval)

	expiresHeader := sip.Expires(Config.MobilePositionExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	event := Event("Catalog;id=2")
	request.AppendHeader(&event)
	response, err := SipUA.SendRequestWithTimeout(5, request)
	if err != nil {
		return err
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("err code %d", response.StatusCode())
	}

	return nil
}

func (d *Device) Broadcast(sourceId, channelId string) sip.ClientTransaction {
	body := fmt.Sprintf(BroadcastFormat, 1, sourceId, channelId)
	request := d.BuildMessageRequest(channelId, body)
	return SipUA.SendRequest(request)
}

func (d *Device) OnKeepalive() {

}

func (d *Device) AddChannels(channels []*Channel) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.Channels == nil {
		d.Channels = make(map[string]*Channel, 5)
	}

	for i, _ := range channels {
		d.Channels[channels[i].DeviceID] = channels[i]
	}
}

func (d *Device) GetChannels() []*Channel {
	d.lock.RLock()
	defer d.lock.RUnlock()

	var channels []*Channel
	for _, channel := range d.Channels {
		channels = append(channels, channel)
	}

	return channels
}

func (d *Device) RemoveChannel(id string) *Channel {
	d.lock.Lock()
	defer d.lock.Unlock()

	if channel, ok := d.Channels[id]; ok {
		delete(d.Channels, id)
		return channel
	}

	return nil
}

func (d *Device) FindChannel(id string) *Channel {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if channel, ok := d.Channels[id]; ok {
		return channel
	}
	return nil
}

func (d *Device) UpdateChannel(id string, event string) {
	d.lock.RLock()
	defer d.lock.RUnlock()
}

func (d *Device) BuildCatalogRequest() (sip.Request, error) {
	body := fmt.Sprintf(CatalogFormat, "1", d.ID)
	request := d.BuildMessageRequest(d.ID, body)
	return request, nil
}

func (d *Device) NewSIPRequestBuilderWithTransport() *sip.RequestBuilder {
	builder := sip.NewRequestBuilder()
	hop := sip.ViaHop{
		Transport: d.Transport,
	}

	builder.AddVia(&hop)
	return builder
}

func (d *Device) NewRequestBuilder(method sip.RequestMethod, fromUser, realm, toUser string) *sip.RequestBuilder {
	builder := d.NewSIPRequestBuilderWithTransport()
	builder.SetMethod(method)

	host, p, _ := net.SplitHostPort(d.RemoteAddr)
	port, _ := strconv.Atoi(p)
	sipPort := sip.Port(port)

	requestUri := &sip.SipUri{
		FUser: sip.String{Str: toUser},
		FHost: host,
		FPort: &sipPort,
	}

	builder.SetRecipient(requestUri)

	fromAddress := &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: fromUser},
			FHost: realm,
		},
	}

	fromAddress.Params = sip.NewParams().Add("tag", sip.String{Str: GenerateTag()})
	builder.SetFrom(fromAddress)
	builder.SetTo(&sip.Address{
		Uri: requestUri,
	})

	return builder
}

func (d *Device) BuildInviteRequest(sessionName, channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	builder := d.NewRequestBuilder(sip.INVITE, Config.SipId, Config.SipContactAddr, channelId)
	sdp := BuildSDP(Config.SipId, sessionName, ip, port, startTime, stopTime, setup, speed, ssrc)
	builder.SetContentType(&SDPMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(sdp)
	request, err := builder.Build()
	if err != nil {
		return nil, err
	}

	var subjectHeader = Subject(channelId + ":" + d.ID + "," + Config.SipId + ":" + ssrc)
	request.AppendHeader(subjectHeader)

	return request, err
}

func (d *Device) BuildLiveRequest(channelId, ip string, port uint16, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Play", channelId, ip, port, "0", "0", setup, 0, ssrc)
}

func (d *Device) BuildPlaybackRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Playback", channelId, ip, port, startTime, stopTime, setup, 0, ssrc)
}

func (d *Device) BuildDownloadRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Download", channelId, ip, port, startTime, stopTime, setup, speed, ssrc)
}

// CreateDialogRequestFromAnswer 根据invite的应答创建Dialog请求
// 应答的to头域需携带tag

func CreateDialogRequestFromAnswer(message sip.Response, uas bool, remoteAddr string) sip.Request {
	from, _ := message.From()
	to, _ := message.To()
	id, _ := message.CallID()

	requestLine := &sip.SipUri{}
	requestLine.SetUser(from.Address.User())
	host, port, _ := net.SplitHostPort(remoteAddr)
	portInt, _ := strconv.Atoi(port)
	sipPort := sip.Port(portInt)
	requestLine.SetHost(host)
	requestLine.SetPort(&sipPort)

	seq, _ := message.CSeq()

	builder := NewSIPRequestBuilderWithTransport(message.Transport())
	if uas {
		builder.SetFrom(sip.NewAddressFromToHeader(to))
		builder.SetTo(sip.NewAddressFromFromHeader(from))
	} else {
		builder.SetFrom(sip.NewAddressFromFromHeader(from))
		builder.SetTo(sip.NewAddressFromToHeader(to))
	}

	builder.SetCallID(id)
	builder.SetMethod(sip.BYE)
	builder.SetRecipient(requestLine)
	builder.SetSeqNo(uint(seq.SeqNo + 1))
	request, err := builder.Build()
	if err != nil {
		panic(err)
	}

	return request
}

func (d *Device) CreateDialogRequestFromAnswer(message sip.Response, uas bool) sip.Request {
	return CreateDialogRequestFromAnswer(message, uas, d.RemoteAddr)
}
