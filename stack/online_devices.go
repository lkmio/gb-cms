package stack

import (
	"gb-cms/dao"
	"gb-cms/log"
	"sync"
	"time"
)

var (
	OnlineDeviceManager = NewOnlineDeviceManager()
)

type onlineDeviceManager struct {
	lock    sync.RWMutex
	devices map[string]time.Time
}

func (m *onlineDeviceManager) Add(deviceId string, t time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.devices[deviceId] = t
}

func (m *onlineDeviceManager) Remove(deviceId string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.devices, deviceId)
}

func (m *onlineDeviceManager) Find(deviceId string) (time.Time, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	t, ok := m.devices[deviceId]
	return t, ok
}

func (m *onlineDeviceManager) Count() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.devices)
}

func (m *onlineDeviceManager) GetDeviceIds() []string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ids := make([]string, 0, len(m.devices))
	for id := range m.devices {
		ids = append(ids, id)
	}
	return ids
}

func (m *onlineDeviceManager) Start(interval time.Duration, keepalive time.Duration, OnExpires func(platformId int, deviceId string)) {
	// 精度有偏差
	var timer *time.Timer
	timer = time.AfterFunc(interval, func() {
		now := time.Now()
		m.lock.Lock()
		defer m.lock.Unlock()
		for deviceId, t := range m.devices {
			if now.Sub(t) < keepalive {
				continue
			}

			delete(m.devices, deviceId)
			go OnExpires(0, deviceId)
		}

		timer.Reset(interval)
	})
}

func NewOnlineDeviceManager() *onlineDeviceManager {
	return &onlineDeviceManager{
		devices: make(map[string]time.Time),
	}
}

// OnExpires Redis设备ID到期回调
func OnExpires(db int, id string) {
	log.Sugar.Infof("设备心跳过期 device: %s", id)

	device, _ := dao.Device.QueryDevice(id)
	if device == nil {
		log.Sugar.Errorf("设备不存在 device: %s", id)
		return
	}

	(&Device{DeviceModel: device}).Close()
}
