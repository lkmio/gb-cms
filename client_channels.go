package main

import "sync"

var (
	// DeviceChannelsManager 目前只用作模拟多个国标客户端. 设备接入和级联都会走数据库
	DeviceChannelsManager *DeviceChannels
)

type DeviceChannels struct {
	channels map[string][]*Channel
	lock     sync.RWMutex
}

func (d *DeviceChannels) AddChannel(deviceId string, channel *Channel) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.channels == nil {
		d.channels = make(map[string][]*Channel, 5)
	}

	channels := d.channels[deviceId]
	channels = append(channels, channel)
	d.channels[deviceId] = channels
}

func (d *DeviceChannels) FindChannels(deviceId string) []*Channel {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.channels[deviceId]
}
