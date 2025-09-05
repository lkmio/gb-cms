package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
	"strings"
)

// PlatformModel 数据库表结构
type PlatformModel struct {
	GBModel
	common.SIPUAOptions
	Enable   bool // 启用/禁用
	ShareAll bool // 级联所有通道
}

func (g *PlatformModel) TableName() string {
	return "lkm_platform"
}

type PlatformChannelModel struct {
	GBModel
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
	PID       uint   `json:"pid"` // 级联设备数据库ID
}

func (d *PlatformChannelModel) TableName() string {
	return "lkm_platform_channel"
}

type daoPlatform struct {
}

func (d *daoPlatform) LoadPlatforms() ([]*PlatformModel, error) {
	var platforms []*PlatformModel
	tx := db.Find(&platforms)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return platforms, nil
}

func (d *daoPlatform) QueryPlatformByAddr(addr string) (*PlatformModel, error) {
	var platform PlatformModel
	tx := db.Where("server_addr =?", addr).First(&platform)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &platform, nil
}

func (d *daoPlatform) SavePlatform(platform *PlatformModel) error {
	var old PlatformModel
	tx := db.Where("server_addr =?", platform.ServerAddr).First(&old)
	if tx.Error == nil {
		platform.ID = old.ID
	}
	return db.Save(platform).Error
}

func (d *daoPlatform) DeletePlatformByAddr(addr string) error {
	// 删除绑定的通道
	return db.Where("server_addr =?", addr).Unscoped().Delete(&PlatformModel{}).Error
}

func (d *daoPlatform) UpdatePlatform(platform *PlatformModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(platform).Error
	})
}

func (d *daoPlatform) UpdateOnlineStatus(status common.OnlineStatus, addr string) error {
	return db.Model(&PlatformModel{}).Where("server_addr =?", addr).Update("status", status).Error
}

func (d *daoPlatform) BindChannels(pid int, channels []string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		for _, channel := range channels {
			ids := strings.Split(channel, ":")
			var old PlatformChannelModel
			// 检查是否已经绑定
			tx.Where("device_id =? and channel_id =? and p_id =?", ids[0], ids[1], pid).First(&old)
			if old.ID != 0 {
				continue
			}

			// 检查通道是否存在
			_, err := Channel.QueryChannel(ids[0], ids[1])
			if err != nil {
				continue
			}

			// 插入绑定关系
			_ = tx.Create(&PlatformChannelModel{
				DeviceID:  ids[0],
				ChannelID: ids[1],
				PID:       uint(pid),
			})
		}
		return nil
	})
}

func (d *daoPlatform) UnbindChannels(pid int, channels []string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		for _, channel := range channels {
			ids := strings.Split(channel, ":")
			tx.Unscoped().Delete(&PlatformChannelModel{}, "device_id =? and channel_id =? and p_id =?", ids[0], ids[1], pid)
		}
		return nil
	})
}

func (d *daoPlatform) QueryPlatformChannel(addr string, channelId string) (string, *ChannelModel, error) {
	model, err := d.QueryPlatformByAddr(addr)
	if err != nil {
		return "", nil, err
	}

	if model.ShareAll {
		channel, _ := Channel.QueryChannelByCustomID(channelId)
		if channel != nil {
			return channel.RootID, channel, nil
		}

		channels, err := Channel.QueryChannelsByChannelID(channelId)
		if err != nil {
			return "", nil, err
		}
		return channels[0].RootID, channels[0], nil
	}

	var platformChannel PlatformChannelModel
	tx := db.Model(&PlatformChannelModel{}).Where("channel_id =? and p_id =?", channelId, model.ID).First(&platformChannel)
	if tx.Error != nil {
		return "", nil, tx.Error
	}

	// 优先查询自定义通道
	channel, _ := Channel.QueryChannelByCustomID(channelId)
	if channel != nil {
		return channel.RootID, channel, nil
	}

	tx = db.Where("root_id =? and device_id =?", platformChannel.DeviceID, platformChannel.ChannelID).First(&channel)
	if tx.Error != nil {
		return "", nil, tx.Error
	}

	return channel.RootID, channel, nil
}

func (d *daoPlatform) QueryPlatformChannels(addr string) ([]*ChannelModel, error) {
	model, err := d.QueryPlatformByAddr(addr)
	if err != nil {
		return nil, err
	}

	// 返回所有通道
	if model.ShareAll {
		channels, _, _ := Channel.QueryChannels("", "", -1, -1, "", "", "", "", false)
		return channels, nil
	}

	var platformChannels []*PlatformChannelModel
	tx := db.Where("p_id =?", model.ID).Find(&platformChannels)
	if tx.Error != nil {
		return nil, tx.Error
	}

	var channels []*ChannelModel
	for _, platformChannel := range platformChannels {
		queryChannel, err := Channel.QueryChannel(platformChannel.DeviceID, platformChannel.ChannelID)
		if err != nil {
			continue
		}
		channels = append(channels, queryChannel)
	}

	return channels, nil
}

func (d *daoPlatform) QueryPlatforms(page, size int, keyword, enable, status string) ([]*PlatformModel, int, error) {
	var platforms []*PlatformModel
	var total int64
	query := db.Model(&PlatformModel{})
	if keyword != "" {
		query = query.Where("username like ?", "%"+keyword+"%")
	}

	if enable == "true" {
		query = query.Where("enable = ?", 1)
	} else if enable == "false" {
		query = query.Where("enable = ?", 0)
	}

	if status == "true" {
		query = query.Where("status = ?", "ON")
	} else if status == "false" {
		query = query.Where("status = ?", "OFF")
	}

	query.Count(&total)
	query.Offset((page - 1) * size).Limit(size).Find(&platforms)
	return platforms, int(total), nil
}

func (d *daoPlatform) UpdateEnable(id int, enable bool) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&PlatformModel{}).Where("id =?", id).Update("enable", enable).Error
	})
}

func (d *daoPlatform) QueryPlatformByID(id int) (*PlatformModel, error) {
	var platform PlatformModel
	tx := db.Where("id =?", id).First(&platform)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &platform, nil
}

func (d *daoPlatform) DeletePlatformByID(id int) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Unscoped().Delete(&PlatformModel{}, id).Error
	})
}

// QueryPlatformChannelList 查询级联设备通道列表
func (d *daoPlatform) QueryPlatformChannelList(id int) ([]*ChannelModel, int, error) {
	var platformChannels []*PlatformChannelModel
	tx := db.Where("p_id =?", id).Find(&platformChannels)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	// 查询通道总数
	count, err := d.QueryPlatformChannelCount(id)
	if err != nil {
		return nil, 0, err
	}

	var channels []*ChannelModel
	for _, platformChannel := range platformChannels {
		channel, err := Channel.QueryChannel(platformChannel.DeviceID, platformChannel.ChannelID)
		if err == nil {
			channels = append(channels, channel)
		}
	}

	return channels, count, nil
}

// QueryPlatformChannelCount 查询级联设备的通道总数
func (d *daoPlatform) QueryPlatformChannelCount(id int) (int, error) {
	var total int64
	tx := db.Model(&PlatformChannelModel{}).Where("p_id =?", id).Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}

	return int(total), nil
}

// QueryPlatformChannelExist 查询某个通道是否绑定到某个级联设备
func (d *daoPlatform) QueryPlatformChannelExist(pid int, deviceId, channelId string) (bool, error) {
	var total int64
	tx := db.Model(&PlatformChannelModel{}).Where("p_id =? and device_id =? and channel_id =?", pid, deviceId, channelId).Count(&total)
	if tx.Error != nil {
		return false, tx.Error
	}

	return total > 0, nil
}

// DeletePlatformChannels 删除级联设备的所有通道
func (d *daoPlatform) DeletePlatformChannels(id int) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Unscoped().Delete(&PlatformChannelModel{}, "p_id =?", id).Error
	})
}

// SetShareAllChannel 设置级联设备是否分享所有通道
func (d *daoPlatform) SetShareAllChannel(id int, shareAll bool) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&PlatformModel{}).Where("id =?", id).Update("share_all", shareAll).Error
	})
}
