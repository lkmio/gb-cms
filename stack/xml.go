package stack

import (
	"encoding/xml"
	"gb-cms/dao"
)

type BaseMessageGetter interface {
	GetDeviceID() string
	GetCmdType() string
	GetSN() int
}

type BaseMessage struct {
	CmdType  string `xml:"CmdType"`
	SN       int    `xml:"SN"`
	DeviceID string `xml:"DeviceID"`
}

func (b BaseMessage) GetDeviceID() string {
	return b.DeviceID
}

func (b BaseMessage) GetCmdType() string {
	return b.CmdType
}

func (b BaseMessage) GetSN() int {
	return b.SN
}

type DeviceList struct {
	Num     int                 `xml:"Num,attr"`
	Devices []*dao.ChannelModel `xml:"Item"`
}

type ExtendedInfo struct {
	Info string `xml:"Info,omitempty"`
}

type BaseResponse struct {
	XMLName xml.Name `xml:"Response"`
	BaseMessage
	Result string `xml:"Result,omitempty"`
	ExtendedInfo
}

type CatalogResponse struct {
	BaseResponse
	SumNum     int        `xml:"SumNum"`
	DeviceList DeviceList `xml:"DeviceList"`
}

type DeviceInfoResponse struct {
	BaseResponse
	DeviceName   string `xml:"DeviceName,omitempty"`
	Manufacturer string `xml:"Manufacturer,omitempty"`
	Model        string `xml:"Model,omitempty"`
	Firmware     string `xml:"Firmware,omitempty"`
	Channel      string `xml:"Channel,omitempty"` //通道数
}

type DeviceStatusResponse struct {
	BaseResponse
	Online     string `xml:"Online"` //ONLINE/OFFLINE
	Status     string `xml:"Status"` //OK/ERROR
	Reason     string `xml:"Reason"` //OK/ERROR
	Encode     string `xml:"Encode"` //ON/OFF
	Record     string `xml:"Record"` //ON/OFF
	DeviceTime string `xml:"DeviceTime"`
}
