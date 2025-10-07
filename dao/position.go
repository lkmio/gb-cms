package dao

import (
	"gorm.io/gorm"
	"time"
)

const (
	PositionSourceSubscribe = iota + 1
	PositionSourceAlarm
	PositionSourceChannel
)

type PositionModel struct {
	GBModel
	DeviceID  string
	ChannelID string
	Longitude float64
	Latitude  float64
	Speed     *string
	Direction *string
	Altitude  *string
	Time      string
	Source    int // 来源
}

func (p *PositionModel) TableName() string {
	return "lkm_position"
}

type daoPosition struct {
}

func (d *daoPosition) SavePosition(position *PositionModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Create(position).Error
	})
}

func (d *daoPosition) DeleteExpired(time time.Time) error {
	// 删除过期的位置记录
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("created_at < ?", time).Delete(&PositionModel{}).Unscoped().Error
	})
}
