package stack

import (
	"fmt"
	"gb-cms/common"
	"github.com/ghettovoice/gosip/sip"
)

const (
	RTSPBodyFormat = "PLAY RTSP/1.0\r\n" +
		"CSeq: %d\r\n" +
		"Scale: %.1f\r\n"
)

func (d *Device) ScalePlayback(dialog sip.Request, speed float64) {
	infoRequest := CreateRequestFromDialog(dialog, sip.INFO)
	sn := GetSN()
	body := fmt.Sprintf(RTSPBodyFormat, sn, speed)
	infoRequest.SetBody(body, true)
	infoRequest.RemoveHeader("Content-Type")
	infoRequest.AppendHeader(&RTSPMessageType)
	infoRequest.RemoveHeader("Contact")
	infoRequest.AppendHeader(GlobalContactAddress.AsContactHeader())

	// 替換到device的真實地址
	recipient := infoRequest.Recipient()
	if uri, ok := recipient.(*sip.SipUri); ok {
		sipPort := sip.Port(d.RemotePort)
		uri.FHost = d.RemoteIP
		uri.FPort = &sipPort
	}

	common.SipStack.SendRequest(infoRequest)
}
