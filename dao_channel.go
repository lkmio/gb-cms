package main

import "gorm.io/gorm"

type DaoChannel interface {
	SaveChannel(deviceId string, channel *Channel) error

	UpdateChannelStatus(deviceId, channelId, status string) error

	QueryChannel(deviceId string, channelId string) (*Channel, error)

	QueryChannels(deviceId string, page, size int) ([]*Channel, int, error)

	QueryChanelCount(deviceId string) (int, error)

	QueryOnlineChanelCount(deviceId string) (int, error)
}

type daoChannel struct {
}

func (d *daoChannel) SaveChannel(deviceId string, channel *Channel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old Channel
		if db.Select("id").Where("parent_id =? and device_id =?", deviceId, channel.DeviceID).Take(&old).Error == nil {
			channel.ID = old.ID
		}
		return tx.Save(channel).Error
	})
}

func (d *daoChannel) UpdateChannelStatus(deviceId, channelId, status string) error {
	return db.Model(&Channel{}).Where("parent_id =? and device_id =?", deviceId, channelId).Update("status", status).Error
}

func (d *daoChannel) QueryChannel(deviceId string, channelId string) (*Channel, error) {
	var channel Channel
	tx := db.Where("parent_id =? and device_id =?", deviceId, channelId).Take(&channel)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &channel, nil
}

func (d *daoChannel) QueryChannels(deviceId string, page, size int) ([]*Channel, int, error) {
	var channels []*Channel
	tx := db.Limit(size).Offset((page-1)*size).Where("parent_id =?", deviceId).Find(&channels)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	var total int64
	tx = db.Model(&Channel{}).Where("parent_id =?", deviceId).Count(&total)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	return channels, int(total), nil
}

func (d *daoChannel) QueryChanelCount(deviceId string) (int, error) {
	var total int64
	tx := db.Model(&Channel{}).Where("parent_id =?", deviceId).Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}

func (d *daoChannel) QueryOnlineChanelCount(deviceId string) (int, error) {
	var total int64
	tx := db.Model(&Channel{}).Where("parent_id =? and status =?", deviceId, "ON").Count(&total)
	if tx.Error != nil {
		return 0, tx.Error
	}
	return int(total), nil
}
