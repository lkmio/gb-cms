package main

import (
	"encoding/xml"
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
)

var (
	XmlMessageType sip.ContentType = "Application/MANSCDP+xml"

	SDPMessageType sip.ContentType = "application/sdp"
)

type DBDevice struct {
	Id         string             `json:"id"`
	Name       string             `json:"name"`
	RemoteAddr string             `json:"remote_addr"`
	Protocol   string             `json:"protocol"`
	Status     string             `xml:"Status,omitempty"` //在线状态 ON-在线/OFF-离线
	Channels   map[string]Channel `json:"channels"`
}

type Channel struct {
	DeviceID     string `xml:"DeviceID"`
	Name         string `xml:"Name,omitempty"`
	Manufacturer string `xml:"Manufacturer,omitempty"`
	Model        string `xml:"Model,omitempty"`
	Owner        string `xml:"Owner,omitempty"`
	CivilCode    string `xml:"CivilCode,omitempty"`
	Block        string `xml:"Block,omitempty"`
	Address      string `xml:"Address,omitempty"`
	Parental     int    `xml:"Parental,omitempty"`
	ParentID     string `xml:"ParentID,omitempty"`
	SafetyWay    int    `xml:"SafetyWay,omitempty"`
	RegisterWay  int    `xml:"RegisterWay,omitempty"`
	CertNum      string `xml:"CertNum,omitempty"`
	Certifiable  int    `xml:"Certifiable,omitempty"`
	ErrCode      int    `xml:"ErrCode,omitempty"`
	EndTime      string `xml:"EndTime,omitempty"`
	Secrecy      string `xml:"Secrecy,omitempty"`
	IPAddress    string `xml:"IPAddress,omitempty"`
	Port         int    `xml:"Port,omitempty"`
	Password     string `xml:"Password,omitempty"`
	Status       string `xml:"Status,omitempty"`
	Longitude    string `xml:"Longitude,omitempty"`
	Latitude     string `xml:"Latitude,omitempty"`
}

type DeviceList struct {
	Num     int       `xml:"Num,attr"`
	Devices []Channel `xml:"Item"`
}

type QueryCatalogResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	SumNum     int        `xml:"SumNum"`
	DeviceList DeviceList `xml:"DeviceList"`
}

func (d *DBDevice) BuildCatalogRequest() (sip.Request, error) {
	body := fmt.Sprintf(CatalogFormat, "1", d.Id)
	return d.BuildMessageRequest(d.Id, body)
}

func (d *DBDevice) BuildMessageRequest(to, body string) (sip.Request, error) {
	builder := d.NewRequestBuilder(sip.MESSAGE, Config.SipId, Config.SipContactAddr, to)
	builder.SetContentType(&XmlMessageType)
	builder.SetBody(body)
	return builder.Build()
}

func (d *DBDevice) NewRequestBuilder(method sip.RequestMethod, from, realm, to string) *sip.RequestBuilder {
	builder := sip.NewRequestBuilder()
	builder.SetMethod(method)

	host, p, _ := net.SplitHostPort(d.RemoteAddr)
	port, _ := strconv.Atoi(p)
	sipPort := sip.Port(port)

	requestUri := &sip.SipUri{
		FUser: sip.String{Str: to},
		FHost: host,
		FPort: &sipPort,
	}

	builder.SetRecipient(requestUri)

	fromAddress := &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: from},
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

func (d *DBDevice) BuildSDP(userName, sessionName, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) string {
	format := "v=0\r\n" +
		"o=%s 0 0 IN IP4 %s\r\n" +
		"s=%s\r\n" +
		"c=IN IP4 %s\r\n" +
		"t=%s %s\r\n" +
		"m=video %d %s 96\r\n" +
		"a=%s\r\n" +
		"a=rtpmap:96 PS/90000\r\n"

	tcpFormat := "a=setup:%s\r\n" +
		"a=connection:new\r\n"

	var tcp bool
	var mediaProtocol string
	if "active" == setup || "passive" == setup {
		mediaProtocol = "TCP/RTP/AVP"
		tcp = true
	} else {
		mediaProtocol = "RTP/AVP"
	}

	sdp := fmt.Sprintf(format, userName, ip, sessionName, ip, startTime, stopTime, port, mediaProtocol, "recvonly")
	if tcp {
		sdp += fmt.Sprintf(tcpFormat, setup)
	}

	if speed > 0 {
		sdp += fmt.Sprintf("a=downloadspeed:%d\r\n", speed)
	}

	sdp += fmt.Sprintf("y=%s\r\n", ssrc)
	return sdp
}

func (d *DBDevice) BuildInviteRequest(sessionName, channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	builder := d.NewRequestBuilder(sip.INVITE, Config.SipId, Config.SipContactAddr, channelId)
	sdp := d.BuildSDP(Config.SipId, sessionName, ip, port, startTime, stopTime, setup, speed, ssrc)
	builder.SetContentType(&SDPMessageType)
	builder.SetContact(globalContactAddress)
	builder.SetBody(sdp)
	request, err := builder.Build()
	if err != nil {
		return nil, err
	}

	var subjectHeader = Subject(channelId + ":" + channelId + "," + Config.SipId + ":" + ssrc)
	request.AppendHeader(subjectHeader)

	return request, err
}

func (d *DBDevice) BuildLiveRequest(channelId, ip string, port uint16, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Play", channelId, ip, port, "0", "0", setup, 0, ssrc)
}

func (d *DBDevice) BuildPlaybackRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Playback", channelId, ip, port, startTime, stopTime, setup, 0, ssrc)
}

func (d *DBDevice) BuildDownloadRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Download", channelId, ip, port, startTime, stopTime, setup, speed, ssrc)
}

// CreateDialogRequestFromAnswer 根据invite的应答创建Dialog请求
// 应答的to头域需携带tag
func (d *DBDevice) CreateDialogRequestFromAnswer(message sip.Response, uas bool) sip.Request {
	from, _ := message.From()
	to, _ := message.To()
	id, _ := message.CallID()

	requestLine := &sip.SipUri{}
	requestLine.SetUser(from.Address.User())
	host, port, _ := net.SplitHostPort(d.RemoteAddr)
	portInt, _ := strconv.Atoi(port)
	sipPort := sip.Port(portInt)
	requestLine.SetHost(host)
	requestLine.SetPort(&sipPort)

	seq, _ := message.CSeq()

	builder := sip.NewRequestBuilder()
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
	build, err := builder.Build()
	if err != nil {
		panic(err)
	}

	return build
}
