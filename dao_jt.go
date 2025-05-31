package main

import (
	"fmt"
	"gorm.io/gorm"
)

// JTDeviceModel 数据库表结构
type JTDeviceModel struct {
	GBModel
	SIPUAOptions
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	SimNumber    string `json:"sim_number"`
}

func (g *JTDeviceModel) TableName() string {
	return "lkm_jt_device"
}

// DaoJTDevice 保存级联和1078设备的sipua参数项
type DaoJTDevice interface {
	LoadDevices() ([]*JTDeviceModel, error)

	UpdateOnlineStatus(status OnlineStatus, username string) error

	QueryDevice(user string) (*JTDeviceModel, error)

	QueryDeviceBySimNumber(simNumber string) (*JTDeviceModel, error)

	ExistDevice(username, simNumber string) bool

	DeleteDevice(username string) error

	SaveDevice(model *JTDeviceModel) error

	UpdateDevice(model *JTDeviceModel) error
}

type daoJTDevice struct {
}

func (d *daoJTDevice) LoadDevices() ([]*JTDeviceModel, error) {
	var devices []*JTDeviceModel
	tx := db.Find(&devices)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return devices, nil
}

func (d *daoJTDevice) UpdateOnlineStatus(status OnlineStatus, username string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&JTDeviceModel{}).Where("username =?", username).Update("status", status).Error
	})
}

func (d *daoJTDevice) ExistDevice(id, simNumber string) bool {
	var device JTDeviceModel
	if db.Where("username =? or sim_number =?", id, simNumber).Select("id").Take(&device).Error == nil {
		return true
	}

	return false
}

func (d *daoJTDevice) QueryDevice(id string) (*JTDeviceModel, error) {
	var device JTDeviceModel
	tx := db.Where("username =?", id).Take(&device)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &device, nil
}

func (d *daoJTDevice) DeleteDevice(id string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("username =?", id).Unscoped().Delete(&JTDeviceModel{}).Error
	})
}

func (d *daoJTDevice) QueryDeviceBySimNumber(simNumber string) (*JTDeviceModel, error) {
	var device JTDeviceModel
	tx := db.Where("sim_number =?", simNumber).Take(&device)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &device, nil
}

func (d *daoJTDevice) SaveDevice(model *JTDeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old JTDeviceModel
		tx = tx.Where("username =? or sim_number =?", model.Username, model.SimNumber).Select("id").First(&old)
		if tx.Error == nil {
			return fmt.Errorf("username or sim number already exists")
		}

		return db.Save(model).Error
	})
}

func (d *daoJTDevice) UpdateDevice(model *JTDeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var old JTDeviceModel
		tx = tx.Where("username =? or sim_number =?", model.Username, model.SimNumber).Select("id").First(&old)
		if tx.Error != nil {
			return tx.Error
		} else {
			model.ID = old.ID
		}

		return db.Save(model).Error
	})
}
