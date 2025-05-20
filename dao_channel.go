package main

import (
	"gorm.io/gorm"
)

type DaoChannel interface {
	SaveChannel(channel *Channel) error

	UpdateChannelStatus(deviceId, channelId, status string) error

	QueryChannel(deviceId string, channelId string) (*Channel, error)

	QueryChannels(deviceId, groupId, string, page, size int) ([]*Channel, int, error)

	QueryChanelCount(deviceId string) (int, error)

	QueryOnlineChanelCount(deviceId string) (int, error)

	QueryChannelByTypeCode(codecs ...int) ([]*Channel, error)
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

func (d *daoChannel) QueryChannel(deviceId string, channelId string) (*Channel, error) {
	var channel Channel
	tx := db.Where("root_id =? and device_id =?", deviceId, channelId).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannels(deviceId, groupId string, page, size int) ([]*Channel, int, error) {
	conditions := map[string]interface{}{}
	conditions["root_id"] = deviceId
	if groupId != "" {
		conditions["group_id"] = groupId
	}

	var channels []*Channel
	tx := db.Limit(size).Offset((page - 1) * size).Where(conditions).Find(&channels)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	var total int64
	tx = db.Model(&Channel{}).Select("id").Where(conditions).Count(&total)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	return channels, int(total), nil
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
