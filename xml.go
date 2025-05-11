package main

import "encoding/xml"

type Channel struct {
	DeviceID     string       `xml:"DeviceID"`
	Name         string       `xml:"Name,omitempty"`
	Manufacturer string       `xml:"Manufacturer,omitempty"`
	Model        string       `xml:"Model,omitempty"`
	Owner        string       `xml:"Owner,omitempty"`
	CivilCode    string       `xml:"CivilCode,omitempty"`
	Block        string       `xml:"Block,omitempty"`
	Address      string       `xml:"Address,omitempty"`
	Parental     string       `xml:"Parental,omitempty"`
	ParentID     string       `xml:"ParentID,omitempty"`
	SafetyWay    string       `xml:"SafetyWay,omitempty"`
	RegisterWay  string       `xml:"RegisterWay,omitempty"`
	CertNum      string       `xml:"CertNum,omitempty"`
	Certifiable  string       `xml:"Certifiable,omitempty"`
	ErrCode      string       `xml:"ErrCode,omitempty"`
	EndTime      string       `xml:"EndTime,omitempty"`
	Secrecy      string       `xml:"Secrecy,omitempty"`
	IPAddress    string       `xml:"IPAddress,omitempty"`
	Port         string       `xml:"Port,omitempty"`
	Password     string       `xml:"Password,omitempty"`
	Status       OnlineStatus `xml:"Status,omitempty"`
	Longitude    string       `xml:"Longitude,omitempty"`
	Latitude     string       `xml:"Latitude,omitempty"`
	SetupType    SetupType    `json:"setup_type,omitempty"`
}

func (d *Channel) Online() bool {
	return d.Status == ON
}

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
	Num     int        `xml:"Num,attr"`
	Devices []*Channel `xml:"Item"`
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
