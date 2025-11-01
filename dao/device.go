package dao

import (
	"gb-cms/common"
	"gb-cms/log"
	"gorm.io/gorm"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultCatalogInterval = 3600 // 默认目录刷新间隔，单位秒
)

type DeviceModel struct {
	GBModel
	DeviceID      string              `json:"device_id" gorm:"index"`
	Name          string              `json:"name" gorm:"index"`
	RemoteIP      string              `json:"remote_ip"`
	RemotePort    int                 `json:"remote_port"`
	RemoteRegion  string              `json:"remote_region"`
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

	CatalogInterval        int       // 录像目录刷新间隔，单位秒, 默认3600每小时刷新
	NextRefreshCatalogTime time.Time `gorm:"type:datetime"` // 下次刷新目录时间
	//ScheduleRecord         [7]uint64 // 录像计划，0-6表示周一至周日，一天的时间刻度用一个uint64表示，从高位开始代表0点，每bit半小时，共占用48位, 1表示录像，0表示不录像
	CatalogSubscribe  bool `json:"catalog_subscribe"`  // 是否开启目录订阅
	AlarmSubscribe    bool `json:"alarm_subscribe"`    // 是否开启报警订阅
	PositionSubscribe bool `json:"position_subscribe"` // 是否开启位置订阅
	Longitude         float64
	Latitude          float64
	DropChannelType   string
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

func (d *DeviceModel) GetSetup() common.SetupType {
	if d.Setup == 0 || d.Setup > common.SetupTypeActive {
		return common.String2SetupType(common.Config.DeviceDefaultMediaTransport)
	}
	return d.Setup
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

	return deviceMap, nil
}

func (d *daoDevice) SaveDevice(device *DeviceModel) error {
	old := DeviceModel{}
	if db.Select("id", "remote_ip", "remote_region").Where("device_id =?", device.DeviceID).Take(&old).Error == nil {
		device.ID = old.ID
	}

	if common.Config.IP2RegionEnable && (old.RemoteRegion == "" || (old.RemoteIP != "" && old.RemoteIP != device.RemoteIP)) {
		region, err := common.IP2Region(device.RemoteIP)
		if err != nil {
			log.Sugar.Errorf("IP2Region failed. err: %s", err.Error())
		}

		device.RemoteRegion = region
	}

	return DBTransaction(func(tx *gorm.DB) error {
		if device.ID == 0 {
			//return tx.Create(&old).Error
			return tx.Save(device).Error
		} else {
			return tx.Model(device).Select("transport", "remote_ip", "remote_port", "status", "register_time", "last_heartbeat", "remote_region").Updates(*device).Error
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
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Update("status", status).Error
	})
}

func (d *daoDevice) RefreshHeartbeat(deviceId string, now time.Time, addr string) error {
	old := DeviceModel{}
	if tx := db.Select("id", "remote_ip", "remote_region").Take(&old, "device_id =?", deviceId); tx.Error != nil {
		return tx.Error
	}

	host, p, _ := net.SplitHostPort(addr)
	var region = old.RemoteRegion
	if common.Config.IP2RegionEnable && (old.RemoteRegion == "" || (old.RemoteIP != "" && old.RemoteIP != host)) {
		var err error
		region, err = common.IP2Region(host)
		if err != nil {
			log.Sugar.Errorf("IP2Region failed. err: %s", err.Error())
		}
	}

	return DBTransaction(func(tx *gorm.DB) error {
		port, _ := strconv.Atoi(p)
		return tx.Model(&DeviceModel{}).Select("last_heartbeat", "status", "remote_ip", "remote_port", "remote_region").Where("device_id =?", deviceId).Updates(&DeviceModel{
			LastHeartbeat: now,
			Status:        common.ON,
			RemoteIP:      host,
			RemotePort:    port,
			RemoteRegion:  region,
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

// QueryDeviceByAddr 根据地址查询设备
func (d *daoDevice) QueryDeviceByAddr(addr string) (*DeviceModel, error) {
	host, p, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(p)
	var device DeviceModel
	tx := db.Where("remote_ip = ? and remote_port = ?", host, port).Take(&device)
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

func (d *daoDevice) Count() (int, error) {
	var count int64
	db.Model(&DeviceModel{}).Count(&count)
	return int(count), nil
}

func (d *daoDevice) UpdateRefreshCatalogTime(deviceId string, now time.Time) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Update("next_refresh_catalog_time", now.Format("2006-01-02 15:04:05")).Error
	})
}

// QueryRefreshCatalogExpiredDevices 查询刷新目录到期的设备列表
func (d *daoDevice) QueryRefreshCatalogExpiredDevices(now time.Time) ([]*DeviceModel, error) {
	var devices []*DeviceModel
	tx := db.Where(
		"(next_refresh_catalog_time < ? OR next_refresh_catalog_time IS NULL) AND status = ?",
		now,
		common.ON,
	).Find(&devices)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return devices, nil
}

// QueryNeedRefreshCatalog 查询设备是否需要刷新目录
func (d *daoDevice) QueryNeedRefreshCatalog(deviceId string, now time.Time) bool {
	var devices int64
	_ = db.Model(&DeviceModel{}).Where(
		"device_id = ? AND (next_refresh_catalog_time < ? OR next_refresh_catalog_time IS NULL)",
		deviceId,
		now,
	).Count(&devices)

	return devices > 0
}

func (d *daoDevice) UpdateCatalogInterval(id string, interval int) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&DeviceModel{}).Where("device_id =?", id).Update("catalog_interval", interval).Error
	})
}

func (d *daoDevice) UpdateDevice(deviceId string, conditions map[string]interface{}) error {
	// 更新下次刷新订阅的时间
	var oldDevice DeviceModel
	var newCatalogInterval int
	if interval, ok := conditions["catalog_interval"]; ok {
		newCatalogInterval = interval.(int)
		_ = db.Model(&oldDevice).Where("device_id =? AND status = ?", deviceId, common.ON).Select("catalog_interval", "next_refresh_catalog_time", "id").Take(&oldDevice).Error
		if oldDevice.ID != 0 && oldDevice.CatalogInterval != 0 {
			oldDevice.NextRefreshCatalogTime = oldDevice.NextRefreshCatalogTime.Add(time.Duration(newCatalogInterval-oldDevice.CatalogInterval) * time.Second)
			conditions["next_refresh_catalog_time"] = oldDevice.NextRefreshCatalogTime.Format("2006-01-02 15:04:05")
		}
	}

	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Updates(conditions).Error
	})
}

// QueryDeviceName 查询设备名
func (d *daoDevice) QueryDeviceName(deviceId string) (string, error) {
	var device DeviceModel
	tx := db.Select("name").Where("device_id =?", deviceId).Take(&device)
	if tx.Error != nil {
		return "", tx.Error
	}

	return device.Name, nil
}

func (d *daoDevice) SetDropChannelType(deviceId string, dropChannelTypes []string) error {
	return DBTransaction(func(tx *gorm.DB) error {
		err := tx.Model(&DeviceModel{}).Where("device_id =?", deviceId).Update("drop_channel_type", strings.Join(dropChannelTypes, ",")).Error
		if err != nil {
			return err
		}

		return Channel.DropChannel(deviceId, dropChannelTypes, tx)
	})
}
