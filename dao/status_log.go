package dao

import (
	"gorm.io/gorm"
	"time"
)

// StatusLogModel 设备上下线记录
type StatusLogModel struct {
	GBModel
	Serial      string `json:"Serial"`
	Code        string `json:"Code"`
	Status      string `json:"Status"`
	Description string `json:"Description"`
	CreatedAt_  string `json:"CreatedAt" gorm:"-"`
}

func (s *StatusLogModel) TableName() string {
	return "lkm_status_log"
}

type daoStatusLog struct {
}

func (s *daoStatusLog) Save(model *StatusLogModel) error {
	return db.Create(model).Error
}

func (s *daoStatusLog) QueryBySerial(serial string, limit int) ([]*StatusLogModel, int, error) {
	// 统计总数
	var count int64
	err := db.Model(&StatusLogModel{}).Where("serial = ?", serial).Select("id").Count(&count).Error
	if err != nil {
		return nil, 0, err
	} else if count < 1 {
		return nil, 0, nil
	}

	var logs []*StatusLogModel
	err = db.Where("serial = ?", serial).Order("created_at desc").Limit(limit).Find(&logs).Error

	// 格式化时间
	for _, log := range logs {
		log.CreatedAt_ = log.CreatedAt.Format("2006-01-02 15:04:05")
	}

	return logs, int(count), err
}

func (s *daoStatusLog) DeleteExpired(time time.Time) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("created_at < ?", time).Delete(&StatusLogModel{}).Unscoped().Error
	})
}
