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
	infoRequest := CreateRequestFromDialog(dialog, sip.INFO, d.RemoteIP, d.RemotePort)
	sn := GetSN()
	body := fmt.Sprintf(RTSPBodyFormat, sn, speed)
	infoRequest.SetBody(body, true)
	infoRequest.RemoveHeader("Content-Type")
	infoRequest.AppendHeader(&RTSPMessageType)
	infoRequest.RemoveHeader("Contact")
	infoRequest.AppendHeader(GlobalContactAddress.AsContactHeader())

	common.SipStack.SendRequest(infoRequest)
}
