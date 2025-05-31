package main

import (
	"encoding/xml"
	"time"
)

// GBModel 解决与Device和Channel的Model变量名冲突
type GBModel struct {
	//gorm.Model
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"-"`
}

type Channel struct {
	GBModel

	// RootID 是设备的根ID, 用于查询设备的所有通道.
	RootID   string `json:"root_id" xml:"-" gorm:"index"` // 根设备ID
	TypeCode int    `json:"-" xml:"-" gorm:"index"`       // 设备类型编码

	// 所在组ID. 扩展的数据库字段, 方便查询某个目录下的设备列表.
	// 如果ParentID不为空, ParentID作为组ID, 如果ParentID为空, BusinessGroupID作为组ID.
	GroupID string `json:"-" xml:"-" gorm:"index"`

	DeviceID        string       `json:"device_id" xml:"DeviceID" gorm:"index"`
	Name            string       `json:"name" xml:"Name,omitempty"`
	Manufacturer    string       `json:"manufacturer" xml:"Manufacturer,omitempty"`
	Model           string       `json:"model" xml:"Model,omitempty"`
	Owner           string       `json:"owner" xml:"Owner,omitempty"`
	CivilCode       string       `json:"civil_code" xml:"CivilCode,omitempty"`
	Block           string       `json:"block" xml:"Block,omitempty"`
	Address         string       `json:"address" xml:"Address,omitempty"`
	Parental        string       `json:"parental" xml:"Parental,omitempty"`
	ParentID        string       `json:"parent_id" xml:"ParentID,omitempty" gorm:"index"` // 父设备ID/系统ID/虚拟目录ID
	BusinessGroupID string       `json:"-" xml:"BusinessGroupID,omitempty" gorm:"index"`
	SafetyWay       string       `json:"safety_way" xml:"SafetyWay,omitempty"`
	RegisterWay     string       `json:"register_way" xml:"RegisterWay,omitempty"`
	CertNum         string       `json:"cert_num" xml:"CertNum,omitempty"`
	Certifiable     string       `json:"certifiable" xml:"Certifiable,omitempty"`
	ErrCode         string       `json:"err_code" xml:"ErrCode,omitempty"`
	EndTime         string       `json:"end_time" xml:"EndTime,omitempty"`
	Secrecy         string       `json:"secrecy" xml:"Secrecy,omitempty"`
	IPAddress       string       `json:"ip_address" xml:"IPAddress,omitempty"`
	Port            string       `json:"port" xml:"Port,omitempty"`
	Password        string       `json:"password" xml:"Password,omitempty"`
	Status          OnlineStatus `json:"status" xml:"Status,omitempty"`
	Longitude       string       `json:"longitude" xml:"Longitude,omitempty"`
	Latitude        string       `json:"latitude" xml:"Latitude,omitempty"`
	SetupType       SetupType    `json:"setup_type,omitempty"`
	ChannelNumber   int          `json:"channel_number" xml:"-"` // 对应1078的通道号
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
