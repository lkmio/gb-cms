package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
	"time"
)

const (
	SipDialogTypeSubscribeCatalog = iota + 1
	SipDialogTypeSubscribeAlarm
	SipDialogTypeSubscribePosition
)

// SipDialogModel 持久化SIP会话
type SipDialogModel struct {
	GBModel
	DeviceID    string // 保存级联上级的会话, 使用server addr作为id
	ChannelID   string
	CallID      string
	Dialog      *common.RequestWrapper `json:"message,omitempty"`
	Type        int
	RefreshTime time.Time
	CSeqNumber  uint32 `gorm:"column:cseq_number"`
}

func (m *SipDialogModel) TableName() string {
	return "lkm_dialog"
}

type daoDialog struct {
}

func (m *daoDialog) QueryDialogs(id string) ([]*SipDialogModel, error) {
	var dialogs []*SipDialogModel
	err := db.Where("device_id = ?", id).Find(&dialogs).Error
	if err != nil {
		return nil, err
	}
	return dialogs, nil
}

func (m *daoDialog) QueryDialogsByType(id string, t int) ([]*SipDialogModel, error) {
	var dialogs []*SipDialogModel
	err := db.Where("device_id = ? and type = ?", id, t).Find(&dialogs).Error
	if err != nil {
		return nil, err
	}
	return dialogs, nil
}

// DeleteDialogs 删除设备下的所有会话
func (m *daoDialog) DeleteDialogs(id string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("device_id = ?", id).Unscoped().Delete(&SipDialogModel{}).Error
	})
}

// DeleteDialogsByType 删除设备下的指定类型会话
func (m *daoDialog) DeleteDialogsByType(id string, t int) (*SipDialogModel, error) {
	var dialog SipDialogModel
	err := DBTransaction(func(tx *gorm.DB) error {
		err := tx.Where("device_id = ? and type = ?", id, t).First(&dialog).Error
		if err != nil {
			return err
		}
		return tx.Unscoped().Delete(&dialog).Error
	})

	return &dialog, err
}

// DeleteDialogByCallID 根据callid删除会话
func (m *daoDialog) DeleteDialogByCallID(callid string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("call_id = ?", callid).Unscoped().Delete(&SipDialogModel{}).Error
	})
}

// Save 保存会话
func (m *daoDialog) Save(dialog *SipDialogModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(dialog).Error
	})
}

// QueryExpiredDialogs 查找即将过期的订阅会话
func (m *daoDialog) QueryExpiredDialogs(now time.Time) ([]*SipDialogModel, error) {
	var dialogs []*SipDialogModel
	err := db.Where("refresh_time <= ?", now).Find(&dialogs).Error
	if err != nil {
		return nil, err
	}
	return dialogs, nil
}

// QueryDialogByCallID 根据callid查询dialog
func (m *daoDialog) QueryDialogByCallID(id string) (*SipDialogModel, error) {
	var dialog SipDialogModel
	err := db.Where("call_id = ?", id).First(&dialog).Error
	if err != nil {
		return nil, err
	}
	return &dialog, nil
}

func (m *daoDialog) UpdateRefreshTime(callid string, refreshTime time.Time) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&SipDialogModel{}).Where("call_id = ?", callid).Update("refresh_time", refreshTime).Error
	})
}

func (m *daoDialog) UpdateCSeqNumber(callid string, cseqNumber uint32) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&SipDialogModel{}).Where("call_id = ?", callid).Update("cseq_number", cseqNumber).Error
	})
}
