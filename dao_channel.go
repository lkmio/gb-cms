package main

import (
	"fmt"
	"gorm.io/gorm"
)

type DaoChannel interface {
	SaveChannel(channel *Channel) error

	UpdateChannelStatus(deviceId, channelId, status string) error

	QueryChannelByID(id uint) (*Channel, error)

	QueryChannel(deviceId string, channelId string) (*Channel, error)

	QueryChannels(deviceId, groupId, string, page, size int) ([]*Channel, int, error)

	QueryChannelsByRootID(rootId string) ([]*Channel, error)

	QueryChannelsByChannelID(channelId string) ([]*Channel, error)

	QueryChanelCount(deviceId string) (int, error)

	QueryOnlineChanelCount(deviceId string) (int, error)

	QueryChannelByTypeCode(codecs ...int) ([]*Channel, error)

	ExistChannel(channelId string) bool

	SaveJTChannel(channel *Channel) error

	ExistJTChannel(simNumber string, channelNumber int) bool

	QueryJTChannelBySimNumber(simNumber string) (*Channel, error)

	DeleteChannel(deviceId string, channelId string) error

	UpdateRootID(rootId, newRootId string) error

	UpdateChannel(channel *Channel) error
}

type daoChannel struct {
}

func (d *daoChannel) SaveChannel(channel *Channel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old Channel
		if db.Select("id").Where("root_id =? and device_id =?", channel.RootID, channel.DeviceID).Take(&old).Error == nil {
			channel.ID = old.ID
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) UpdateChannelStatus(deviceId, channelId, status string) error {
	return db.Model(&Channel{}).Where("root_id =? and device_id =?", deviceId, channelId).Update("status", status).Error
}

func (d *daoChannel) QueryChannelByID(id uint) (*Channel, error) {
	var channel Channel
	tx := db.Where("id =?", id).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannel(deviceId string, channelId string) (*Channel, error) {
	var channel Channel
	tx := db.Where("root_id =? and device_id =?", deviceId, channelId).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannels(deviceId, groupId string, page, size int, status string, keyword string) ([]*Channel, int, error) {
	conditions := map[string]interface{}{}
	conditions["root_id"] = deviceId
	if groupId != "" {
		conditions["group_id"] = groupId
	}
	if status != "" {
		conditions["status"] = status
	}

	cTx := db.Limit(size).Offset((page - 1) * size).Where(conditions)
	if keyword != "" {
		cTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var channels []*Channel
	if tx := cTx.Find(&channels); tx.Error != nil {
		return nil, 0, tx.Error
	}

	countTx := db.Model(&Channel{}).Select("id").Where(conditions)
	if keyword != "" {
		countTx.Where("name like ? or device_id like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if tx := countTx.Count(&total); tx.Error != nil {
		return nil, 0, tx.Error
	}

	// 查询每个通道的子节点通道数量
	for _, channel := range channels {
		// 查询子节点数量
		var subCount int64
		tx := db.Model(&Channel{}).Where("root_id =? and group_id =?", deviceId, channel.DeviceID).Count(&subCount)
		if tx.Error != nil {
			return nil, 0, tx.Error
		}
		channel.SubCount = int(subCount)
	}

	return channels, int(total), nil
}

func (d *daoChannel) QueryChannelsByRootID(rootId string) ([]*Channel, error) {
	var channels []*Channel
	tx := db.Where("root_id =?", rootId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) QueryChanelCount(deviceId string) (int, error) {
	var total int64
	tx := db.Model(&Channel{}).Where("root_id =?", deviceId).Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineChanelCount(deviceId string) (int, error) {
	var total int64
	tx := db.Model(&Channel{}).Where("root_id =? and status =?", deviceId, "ON").Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}

	return int(total), nil
}

func (d *daoChannel) QueryChannelByTypeCode(codecs ...int) ([]*Channel, error) {
	var channels []*Channel
	tx := db.Where("type_code in ?", codecs).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) ExistChannel(channelId string) bool {
	var channel Channel
	if db.Select("id").Where("device_id =?", channelId).Take(&channel).Error == nil {
		return true
	}

	return false
}

func (d *daoChannel) SaveJTChannel(channel *Channel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old Channel
		if tx.Select("id").Where("root_id =? and channel_number =?", channel.RootID, channel.ChannelNumber).Take(&old).Error == nil {
			return fmt.Errorf("channel number %d already exist", channel.ChannelNumber)
		} else if tx.Select("id").Where("device_id =?", channel.DeviceID).Take(&old).Error == nil {
			return fmt.Errorf("channel id %s already exist", channel.DeviceID)
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) DeleteChannel(deviceId string, channelId string) error {
	return db.Where("root_id =? and device_id =?", deviceId, channelId).Unscoped().Delete(&Channel{}).Error
}

func (d *daoChannel) QueryChannelsByChannelID(channelId string) ([]*Channel, error) {
	var channels []*Channel
	tx := db.Where("device_id =?", channelId).Find(&channels)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return channels, nil
}

func (d *daoChannel) UpdateRootID(rootId, newRootId string) error {
	channel := &Channel{
		RootID:   newRootId,
		GroupID:  newRootId,
		ParentID: newRootId,
	}
	return db.Model(channel).Where("root_id =?", rootId).Select("root_id", "group_id", "parent_id").Updates(channel).Error
}

func (d *daoChannel) UpdateChannel(channel *Channel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(channel).Where("id =?", channel.ID).Updates(channel).Error
	})
}
