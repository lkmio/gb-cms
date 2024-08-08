package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/ghettovoice/gosip/sip"
)

const (
	QueryRecordFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>RecordInfo</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<StartTime>%s</StartTime>\r\n" +
		"<EndTime>%s</EndTime>\r\n" +
		"<Type>%s</Type>\r\n" +
		"</Query>\r\n"

	SeekBodyFormat = "PLAY RTSP/1.0\r\n" + "CSeq: %d\r\n" + "Range: npt=%d-\r\n"
)

var (
	RtspMessageType sip.ContentType = "application/RTSP"
)

type QueryRecordInfoResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	CmdType    string     `xml:"CmdType"`
	SN         int        `xml:"SN"`
	DeviceID   string     `xml:"DeviceID"`
	SumNum     int        `xml:"SumNum"`
	DeviceList RecordList `xml:"RecordList"`
}

type RecordList struct {
	Num            int          `xml:"Num,attr"`
	Devices        []RecordInfo `xml:"Item"`
	cancelFunction *context.CancelFunc
}

type RecordInfo struct {
	FileSize       uint64 `xml:"FileSize" json:"fileSize"`
	StartTime      string `xml:"StartTime" json:"startTime"`
	EndTime        string `xml:"EndTime" json:"endTime"`
	FilePath       string `xml:"FilePath" json:"filePath"`
	ResourceType   string `xml:"ResourceType" json:"type"`
	ResourceId     string `xml:"ResourceId" json:"resourceId"`
	RecorderId     string `xml:"RecorderId" json:"recorderId"`
	UserId         string `xml:"UserId" json:"userId"`
	UserName       string `xml:"UserName" json:"userName"`
	ResourceName   string `xml:"ResourceName" json:"resourceName"`
	ResourceLength string `xml:"ResourceLength" json:"resourceLength"`
	ImportTime     string `xml:"ImportTime" json:"importTime"`
	ResourceUrl    string `xml:"ResourceUrl" json:"resourceUrl"`
	Remark         string `xml:"Remark" json:"remark"`
	Level          string `xml:"Level" json:"level"`
	BootTime       string `xml:"BootTime" json:"bootTime"`
	ShutdownTime   string `xml:"ShutdownTime" json:"shutdownTime"`
}

func (d *DBDevice) DoRecordList(channelId, startTime, endTime string, sn int, type_ string) error {
	body := fmt.Sprintf(QueryRecordFormat, sn, channelId, startTime, endTime, type_)
	msg, err := d.BuildMessageRequest(channelId, body)
	if err != nil {
		return err
	}

	SipUA.SendRequest(msg)
	return nil
}

func (d *DBDevice) OnRecord(response *QueryRecordInfoResponse) {
	event := SNManager.FindEvent(response.SN)
	if event == nil {
		Sugar.Errorf("处理录像查询响应失败 SN:%d", response.SN)
		return
	}

	event(response)
}
