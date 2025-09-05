package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
	"net"
	"strconv"
	"time"
)

var (
	DeviceCount int
)

type DeviceModel struct {
	GBModel
	DeviceID      string              `json:"device_id" gorm:"index"`
	Name          string              `json:"name" gorm:"index"`
	RemoteIP      string              `json:"remote_ip"`
	RemotePort    int                 `json:"remote_port"`
	Transport     string              `json:"transport"` // 信令传输模式 UDP/TCP
	Status        common.OnlineStatus `json:"status"`    // 在线状态 ON-在线/OFF-离线
	Manufacturer  string              `json:"manufacturer"`
	UserAgent     string              `json:"user_agent"`
	Model         string              `json:"model"`
	Firmware      string              `json:"firmware"`
	RegisterTime  time.Time           `json:"register_time"`  // 注册时间
	LastHeartbeat time.Time           `json:"last_heartbeat"` // 最后心跳时间

	ChannelsTotal  int `json:"total_channels"`  // 通道总数
	ChannelsOnline int `json:"online_channels"` // 通道在线数量
	Setup          common.SetupType
}

func (d *DeviceModel) TableName() string {
	return "lkm_device"
}

func (d *DeviceModel) Online() bool {
	return d.Status == common.ON
}

func (d *DeviceModel) GetID() string {
	return d.DeviceID
}

type daoDevice struct {
}

func (d *daoDevice) LoadOnlineDevices() (map[string]*DeviceModel, error) {
	//TODO implement me
	panic("implement me")
}

func (d *daoDevice) LoadDevices() (map[string]*DeviceModel, error) {
	var devices []*DeviceModel
	tx := db.Find(&devices)
	if tx.Error != nil {
		return nil, tx.Error
	}

	deviceMap := make(map[string]*DeviceModel)
	for _, device := range devices {
		deviceMap[device.DeviceID] = device
	}

	DeviceCount = len(devices)
	return deviceMap, nil
}

func (d *daoDevice) SaveDevice(device *DeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		old := DeviceModel{}
		if db.Select("id").Where("device_id =?", device.DeviceID).Take(&old).Error == nil {
			device.ID = old.ID
		}

		if device.ID == 0 {
			//return tx.Create(&old).Error
			err := tx.Save(device).Error
			if err == nil {
				DeviceCount++
			}
			return err
		} else {
			return tx.Model(device).Select("Transport", "RemoteIP", "RemotePort", "Status", "RegisterTime", "LastHeartbeat").Updates(*device).Error
		}
	})
}

func (d *daoDevice) UpdateDeviceInfo(deviceId string, device *DeviceModel) error {
	return DBTransaction(func(tx *gorm.DB) error {
		var condition = make(map[string]interface{})
		if device.Manufacturer != "" {
			condition["manufacturer"] = device.Manufacturer
		}
		if device.Model != "" {
			condition["model"] = device.Model
		}
		if device.Firmware != "" {
			condition["firmware"] = device.Firmware
		}
		if device.Name != "" {
			condition["name"] = device.Name
		}
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Updates(condition).Error
	})
}

func (d *daoDevice) UpdateDeviceStatus(deviceId string, status common.OnlineStatus) error {
	return DBTransaction(func(tx *gorm.DB) error {
		if status == common.ON {
			DeviceCount++
		} else if status == common.OFF {
			DeviceCount--
		}
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Update("status", status).Error
	})
}

func (d *daoDevice) RefreshHeartbeat(deviceId string, now time.Time, addr string) error {
	if tx := db.Select("id").Take(&DeviceModel{}, "device_id =?", deviceId); tx.Error != nil {
		return tx.Error
	}
	return DBTransaction(func(tx *gorm.DB) error {
		host, p, _ := net.SplitHostPort(addr)
		port, _ := strconv.Atoi(p)
		return tx.Model(&DeviceModel{}).Select("LastHeartbeat", "Status", "RemoteAddr").Where("device_id =?", deviceId).Updates(&DeviceModel{
			LastHeartbeat: now,
			Status:        common.ON,
			RemoteIP:      host,
			RemotePort:    port,
		}).Error
	})
}

func (d *daoDevice) QueryDevice(id string) (*DeviceModel, error) {
	var device DeviceModel
	tx := db.Where("device_id =?", id).Take(&device)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &device, nil
}

func (d *daoDevice) QueryDevices(page int, size int, status string, keyword string, order string) ([]*DeviceModel, int, error) {
	var cond = make(map[string]interface{})
	if status != "" {
		cond["status"] = status
	}

	devicesTx := db.Where(cond).Limit(size).Offset((page - 1) * size)
	if keyword != "" {
		devicesTx.Where("device_id like ? or name like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var devices []*DeviceModel
	if tx := devicesTx.Order("device_id " + order).Find(&devices); tx.Error != nil {
		return nil, 0, tx.Error
	}

	countTx := db.Where(cond).Model(&DeviceModel{})
	if keyword != "" {
		countTx.Where("device_id like ? or name like ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	if tx := countTx.Count(&total); tx.Error != nil {
		return nil, 0, tx.Error
	}

	for _, device := range devices {
		count, _ := Channel.QueryChanelCount(device.DeviceID, true)
		online, _ := Channel.QueryOnlineChanelCount(device.DeviceID, true)
		device.ChannelsOnline = online
		device.ChannelsTotal = count
	}

	return devices, int(total), nil
}

func (d *daoDevice) UpdateOfflineDevices(deviceIds []string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&DeviceModel{}).Where("device_id in ?", deviceIds).Update("status", common.OFF).Error
	})
}

func (d *daoDevice) ExistDevice(deviceId string) bool {
	var device DeviceModel
	tx := db.Select("id").Where("device_id =?", deviceId).Take(&device)
	if tx.Error != nil {
		return false
	}

	return true
}

func (d *daoDevice) UpdateMediaTransport(deviceId string, setupType common.SetupType) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Update("setup", setupType).Error
	})
}

func (d *daoDevice) DeleteDevice(deviceId string) error {
	err := DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("device_id =?", deviceId).Unscoped().Delete(&DeviceModel{}).Error
	})
	if err != nil {
		return err
	}

	return Channel.DeleteChannels(deviceId)
}

func (d *daoDevice) DeleteDevicesByIP(ip string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("remote_ip =?", ip).Unscoped().Delete(&DeviceModel{}).Error
	})
}

func (d *daoDevice) DeleteDevicesByUA(ua string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("user_agent =?", ua).Unscoped().Delete(&DeviceModel{}).Error
	})
}
