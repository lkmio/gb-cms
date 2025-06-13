package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
)

const (
	EventPresence = "presence" //SIP 的事件通知机制（如 RFC 3856 和 RFC 6665）实现
	//MobilePositionMessageFormat = "<?xml version=\"1.0\"?>\r\n" +
	//	"<Query>\r\n" +
	//	"<CmdType>MobilePosition</CmdType>\r\n" +
	//	"<SN>%s</SN>\r\n" +
	//	"<DeviceID>%s</DeviceID>\r\n" +
	//	"<Interval>%d</Interval>\r\n" +
	//	"</Query>\r\n"
	MobilePositionMessageFormat = "<Query><CmdType>MobilePosition</CmdType><SN>%d</SN><DeviceID>%s</DeviceID><Interval>%d</Interval></Query>"
)

type MobilePositionNotify struct {
	DeviceID  string `xml:"DeviceID"`
	CmdType   string `xml:"CmdType"`
	SN        int    `xml:"SN"`
	Time      string `xml:"Time"`
	Longitude string `xml:"Longitude"`
	Latitude  string `xml:"Latitude"`
	Speed     string `xml:"Speed"`
	Direction string `xml:"Direction"`
	Altitude  string `xml:"Altitude"`
}

func (d *Device) DoSubscribePosition(channelId string) error {
	if channelId == "" {
		channelId = d.DeviceID
	}

	//暂时不考虑级联
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, Config.SipID, Config.SipContactAddr, channelId)
	body := fmt.Sprintf(MobilePositionMessageFormat, 1, channelId, Config.MobilePositionInterval)

	expiresHeader := sip.Expires(Config.MobilePositionExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	event := Event(EventPresence)
	request.AppendHeader(&event)
	response, err := SipStack.SendRequestWithTimeout(5, request)
	if err != nil {
		return err
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("err code %d", response.StatusCode())
	}

	return nil
}

func (d *Device) OnMobilePositionNotify(notify *MobilePositionNotify) {
	Sugar.Infof("收到位置信息 device:%s data:%v", d.DeviceID, notify)
}
