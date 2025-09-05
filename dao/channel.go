package dao

import (
	"fmt"
	"gb-cms/common"
	"gorm.io/gorm"
	"time"
)

// GBModel 解决`Model`变量名与gorm.Model冲突
type GBModel struct {
	ID        uint      `gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"-"`
}

type ChannelModel struct {
	GBModel

	// RootID 是设备的根ID, 用于查询设备的所有通道.
	RootID   string `json:"root_id" xml:"-" gorm:"index"` // 根设备ID
	TypeCode int    `json:"-" xml:"-" gorm:"index"`       // 设备类型编码

	// GroupID 所在组ID. 扩展的数据库字段, 方便查询某个目录下的设备列表.
	// 如果ParentID不为空, ParentID作为组ID, 如果ParentID为空, BusinessGroupID作为组ID.
	GroupID string `json:"-" xml:"-" gorm:"index"`

	DeviceID        string              `json:"device_id" xml:"DeviceID" gorm:"index"`
	Name            string              `json:"name" xml:"Name,omitempty"`
	Manufacturer    string              `json:"manufacturer" xml:"Manufacturer,omitempty"`
	Model           string              `json:"model" xml:"Model,omitempty"`
	Owner           string              `json:"owner" xml:"Owner,omitempty"`
	CivilCode       string              `json:"civil_code" xml:"CivilCode,omitempty"`
	Block           string              `json:"block" xml:"Block,omitempty"`
	Address         string              `json:"address" xml:"Address,omitempty"`
	Parental        string              `json:"parental" xml:"Parental,omitempty"`
	ParentID        string              `json:"parent_id" xml:"ParentID,omitempty" gorm:"index"` // 父设备ID/系统ID/虚拟目录ID
	BusinessGroupID string              `json:"-" xml:"BusinessGroupID,omitempty" gorm:"index"`
	SafetyWay       string              `json:"safety_way" xml:"SafetyWay,omitempty"`
	RegisterWay     string              `json:"register_way" xml:"RegisterWay,omitempty"`
	CertNum         string              `json:"cert_num" xml:"CertNum,omitempty"`
	Certifiable     string              `json:"certifiable" xml:"Certifiable,omitempty"`
	ErrCode         string              `json:"err_code" xml:"ErrCode,omitempty"`
	EndTime         string              `json:"end_time" xml:"EndTime,omitempty"`
	Secrecy         string              `json:"secrecy" xml:"Secrecy,omitempty"`
	IPAddress       string              `json:"ip_address" xml:"IPAddress,omitempty"`
	Port            string              `json:"port" xml:"Port,omitempty"`
	Password        string              `json:"password" xml:"Password,omitempty"`
	Status          common.OnlineStatus `json:"status" xml:"Status,omitempty"`
	Longitude       string              `json:"longitude" xml:"Longitude,omitempty"`
	Latitude        string              `json:"latitude" xml:"Latitude,omitempty"`
	Setup           common.SetupType    `json:"setup,omitempty"`
	ChannelNumber   int                 `json:"channel_number" xml:"-"` // 对应1078的通道号
	SubCount        int                 `json:"-" xml:"-"`              // 子节点数量
	IsDir           bool                `json:"-" xml:"-"`              // 是否是目录
	CustomID        *string             `gorm:"unique"`                 // 自定义通道ID
}

func (d *ChannelModel) TableName() string {
	return "lkm_channel"
}

func (d *ChannelModel) Online() bool {
	return d.Status == common.ON
}

type daoChannel struct {
}

func (d *daoChannel) SaveChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old ChannelModel
		if db.Select("id").Where("root_id =? and device_id =?", channel.RootID, channel.DeviceID).Take(&old).Error == nil {
			channel.ID = old.ID
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) SaveChannels(channels []*ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(channels).Error
	})
}

func (d *daoChannel) UpdateChannelStatus(deviceId, channelId, status string) error {
	return db.Model(&ChannelModel{}).Where("root_id =? and device_id =?", deviceId, channelId).Update("status", status).Error
}

func (d *daoChannel) QueryChannelByID(id uint) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("id =?", id).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannel(deviceId string, channelId string) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("root_id =? and device_id =?", deviceId, channelId).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannels(deviceId, groupId string, page, size int, status string, keyword string, order, sort string, isDir bool) ([]*ChannelModel, int, error) {
	conditions := map[string]interface{}{}
	if deviceId != "" {
		conditions["root_id"] = deviceId
	}

	if groupId != "" {
		conditions["group_id"] = groupId
	}
	if status != "" {
		conditions["status"] = status
	}
	if isDir {
		conditions["is_dir"] = 1
	}

	cTx := db.Where(conditions)

	if page > 0 {
		cTx.Limit(size).Offset((page - 1) * size)
	}
	if keyword != "" {
		cTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var channels []*ChannelModel
	if sort != "iD" {
		if tx := cTx.Order("device_id " + order).Find(&channels); tx.Error != nil {
			return nil, 0, tx.Error
		}
	} else {
		if tx := cTx.Order("id " + order).Find(&channels); tx.Error != nil {
			return nil, 0, tx.Error
		}
	}

	countTx := db.Model(&ChannelModel{}).Select("id").Where(conditions)
	if keyword != "" {
		countTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if tx := countTx.Count(&total); tx.Error != nil {
		return nil, 0, tx.Error
	}

	return channels, int(total), nil
}

func (d *daoChannel) QueryChannelsByRootID(rootId string) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("root_id =?", rootId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) QueryChanelCount(deviceId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =?", deviceId)
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineChanelCount(deviceId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and status =?", deviceId, "ON")
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Count(&total); tx.Error != nil {
		return 0, tx.Error
	}

	return int(total), nil
}

func (d *daoChannel) QueryChannelByTypeCode(codecs ...int) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("type_code in ?", codecs).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) ExistChannel(channelId string) bool {
	var channel ChannelModel
	if db.Select("id").Where("device_id =?", channelId).Take(&channel).Error == nil {
		return true
	}

	return false
}

func (d *daoChannel) SaveJTChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old ChannelModel
		if tx.Select("id").Where("root_id =? and channel_number =?", channel.RootID, channel.ChannelNumber).Take(&old).Error == nil {
			return fmt.Errorf("channel number %d already exist", channel.ChannelNumber)
		} else if tx.Select("id").Where("device_id =?", channel.DeviceID).Take(&old).Error == nil {
			return fmt.Errorf("channel id %s already exist", channel.DeviceID)
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) DeleteChannels(deviceId string) error {
	return db.Where("root_id =?", deviceId).Unscoped().Delete(&ChannelModel{}).Error
}

func (d *daoChannel) DeleteChannel(deviceId string, channelId string) error {
	return db.Where("root_id =? and device_id =?", deviceId, channelId).Unscoped().Delete(&ChannelModel{}).Error
}

func (d *daoChannel) QueryChannelsByChannelID(channelId string) ([]*ChannelModel, error) {
	var channels []*ChannelModel
	tx := db.Where("device_id =?", channelId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

// QueryChannelByCustomID 根据自定义通道ID查询通道
func (d *daoChannel) QueryChannelByCustomID(customID string) (*ChannelModel, error) {
	var channel ChannelModel
	tx := db.Where("custom_id =?", customID).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) UpdateRootID(rootId, newRootId string) error {
	channel := &ChannelModel{
		RootID:   newRootId,
		GroupID:  newRootId,
		ParentID: newRootId,
	}
	return db.Model(channel).Where("root_id =?", rootId).Select("root_id", "group_id", "parent_id").Updates(channel).Error
}

func (d *daoChannel) UpdateChannel(channel *ChannelModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(channel).Where("id =?", channel.ID).Updates(channel).Error
	})
}

func (d *daoChannel) TotalCount() (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) OnlineCount(ids []string) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("status =? and root_id in ?", "ON", ids).Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QuerySubChannelCount(rootId string, groupId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and group_id =?", rootId, groupId)
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}

	if tx = tx.Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineSubChannelCount(rootId string, groupId string, hasDir bool) (int, error) {
	var total int64
	tx := db.Model(&ChannelModel{}).Where("root_id =? and group_id =? and status =?", rootId, groupId, "ON")
	if !hasDir {
		tx.Where("is_dir =?", 0)
	}
	if tx = tx.Count(&total); tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

// UpdateCustomID 更新自定义通道ID
func (d *daoChannel) UpdateCustomID(rootId, channelId string, customID string) error {
	return db.Model(&ChannelModel{}).Where("root_id =? and device_id =?", rootId, channelId).Update("custom_id", customID).Error
}
