package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"strconv"
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

	DeviceInfoFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>DeviceInfo</CmdType>\r\n" +
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

type OnlineStatus string

const (
	ON  = OnlineStatus("ON")
	OFF = OnlineStatus("OFF")
)

func (s OnlineStatus) String() string {
	return string(s)
}

type GBDevice interface {
	GetID() string

	// QueryDeviceInfo 发送查询设备信息命令
	QueryDeviceInfo()

	// QueryCatalog 发送查询目录命令
	QueryCatalog()

	// QueryRecord 发送查询录像命令
	QueryRecord(channelId, startTime, endTime string, sn int, type_ string) error

	//Invite(channel string, setup string)

	// OnInvite 语音广播
	OnInvite(request sip.Request, user string) sip.Response

	// OnBye 设备侧主动挂断
	OnBye(request sip.Request)

	//
	//OnNotifyCatalog()
	//
	//OnNotifyAlarm()

	SubscribePosition(channelId string) error

	//SubscribeCatalog()
	//
	//SubscribeAlarm()

	Broadcast(sourceId, channelId string) sip.ClientTransaction

	// UpdateChannel 订阅目录，通道发生改变
	// 附录P.4.2.2
	// @Params event ON-上线/OFF-离线/VLOST-视频丢失/DEFECT-故障/ADD-增加/DEL-删除/UPDATE-更新
	UpdateChannel(id string, event string)

	Close()
}

type Device struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	RemoteAddr   string       `json:"remote_addr"`
	Transport    string       `json:"transport"`
	Status       OnlineStatus `json:"status"` //在线状态 ON-在线/OFF-离线
	Manufacturer string       `json:"manufacturer"`
	Model        string       `json:"model"`
	Firmware     string       `json:"firmware"`
	RegisterTime int64        `json:"register_time"`

	ChannelsTotal  int `json:"total_channels"`  // 通道总数
	ChannelsOnline int `json:"online_channels"` // 通道在线数量
}

func (d *Device) GetID() string {
	return d.ID
}

func (d *Device) Online() bool {
	return d.Status == ON
}

func (d *Device) BuildMessageRequest(to, body string) sip.Request {
	request, err := BuildMessageRequest(Config.SipId, net.JoinHostPort(GlobalContactAddress.Uri.Host(), GlobalContactAddress.Uri.Port().String()), to, d.RemoteAddr, d.Transport, body)
	if err != nil {
		panic(err)
	}

	return request
}

func (d *Device) QueryDeviceInfo() {
	body := fmt.Sprintf(DeviceInfoFormat, "1", d.ID)
	request := d.BuildMessageRequest(d.ID, body)
	SipUA.SendRequest(request)
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

func (d *Device) UpdateChannel(id string, event string) {

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

func (d *Device) Close() {
	// 更新在数据库中的状态
	d.Status = OFF
	if err := DB.SaveDevice(d); err != nil {
		Sugar.Errorf("更新设备在线状态失败 err: %s device: %s ", err.Error(), d.ID)
	}

	// 释放所有推流
	all := StreamManager.All()
	var streams []*Stream
	for _, stream := range all {
		if d.ID == stream.ID.DeviceID() {
			streams = append(streams, stream)
		}
	}

	for _, stream := range streams {
		stream.Close(true, true)
	}
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
