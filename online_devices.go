package main

import (
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
