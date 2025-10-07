package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
)

const (
	EventPresence               = "presence" //SIP 的事件通知机制（如 RFC 3856 和 RFC 6665）实现
	MobilePositionMessageFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>MobilePosition</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<Interval>%d</Interval>\r\n" +
		"</Query>\r\n"

	MobilePositionMessageFormatUnsubscribe = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>MobilePosition</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"</Query>\r\n"

	//MobilePositionMessageFormat = "<Query>" +
	//	"<CmdType>MobilePosition</CmdType>" +
	//	"<SN>%d</SN>" +
	//	"<DeviceID>%s</DeviceID>" +
	//	"<Interval>%d</Interval>" +
	//	"</Query>"
)

type MobilePositionNotify struct {
	DeviceID  string  `xml:"DeviceID"`
	CmdType   string  `xml:"CmdType"`
	SN        int     `xml:"SN"`
	Time      string  `xml:"Time"`
	Longitude float64 `xml:"Longitude"`
	Latitude  float64 `xml:"Latitude"`
	Speed     *string `xml:"Speed"`
	Direction *string `xml:"Direction"`
	Altitude  *string `xml:"Altitude"`
}

func (d *Device) SubscribePosition() error {
	channelId := d.DeviceID

	// 暂时不考虑级联
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, common.Config.SipID, common.Config.SipContactAddr, channelId)
	body := fmt.Sprintf(MobilePositionMessageFormat, GetSN(), channelId, common.Config.MobilePositionInterval)

	expiresHeader := sip.Expires(common.Config.SubscribeExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	err = SendSubscribeMessage(d.DeviceID, request, dao.SipDialogTypeSubscribePosition, EventPresence)
	if err != nil {
		log.Sugar.Errorf("订阅位置失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}

	return err
}

func (d *Device) UnsubscribePosition() {
	body := fmt.Sprintf(MobilePositionMessageFormatUnsubscribe, GetSN(), d.DeviceID)
	err := Unsubscribe(d.DeviceID, dao.SipDialogTypeSubscribePosition, EventPresence, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("取消订阅位置失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}

func (d *Device) RefreshSubscribePosition() {
	body := fmt.Sprintf(MobilePositionMessageFormat, GetSN(), d.DeviceID, common.Config.MobilePositionInterval)
	err := RefreshSubscribe(d.DeviceID, dao.SipDialogTypeSubscribePosition, EventPresence, common.Config.SubscribeExpires, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("刷新位置订阅失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}
