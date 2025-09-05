package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
)

// JTDeviceModel 数据库表结构
type JTDeviceModel struct {
	GBModel
	// SIPUAOptions

	Name              string              `json:"name"`                        // display name, 国标DeviceInfo消息中的Name
	Username          string              `json:"username" gorm:"uniqueIndex"` // 用户名
	SeverID           string              `json:"server_id"`                   // 上级ID, 必选. 作为主键, 不能重复.
	ServerAddr        string              `json:"server_addr"`                 // 上级地址, 必选
	Transport         string              `json:"transport"`                   // 上级通信方式, UDP/TCP
	Password          string              `json:"password"`                    // 密码
	RegisterExpires   int                 `json:"register_expires"`            // 注册有效期
	KeepaliveInterval int                 `json:"keepalive_interval"`          // 心跳间隔
	Status            common.OnlineStatus `json:"status"`                      // 在线状态

	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	SimNumber    string `json:"sim_number" gorm:"uniqueIndex"`
}

func (g *JTDeviceModel) TableName() string {
	return "lkm_jt_device"
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

func (d *daoJTDevice) UpdateOnlineStatus(status common.OnlineStatus, username string) error {
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
		err := tx.Where("username =?", id).Unscoped().Delete(&JTDeviceModel{}).Error
		if err != nil {
			return err
		}
		return tx.Where("root_id =?", id).Unscoped().Delete(&ChannelModel{}).Error
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

func (d *daoJTDevice) QueryDeviceByID(id uint) (*JTDeviceModel, error) {
	var device JTDeviceModel
	tx := db.Where("id =?", id).Take(&device)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return &device, nil
}

func (d *daoJTDevice) SaveDevice(model *JTDeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return db.Create(model).Error
	})
}

func (d *daoJTDevice) UpdateDevice(model *JTDeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return db.Save(model).Error
	})
}

func (d *daoJTDevice) QueryDevices(page int, size int) ([]*JTDeviceModel, int, error) {
	var devices []*JTDeviceModel
	tx := db.Limit(size).Offset((page - 1) * size).Find(&devices)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	var total int64
	tx = db.Model(&JTDeviceModel{}).Count(&total)
	if tx.Error != nil {
		return nil, 0, tx.Error
	}

	return devices, int(total), nil
}
