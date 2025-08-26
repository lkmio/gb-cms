package stack

import (
	"gb-cms/dao"
	"sync"
)

var (
	// DeviceChannelsManager 目前只用作模拟多个国标客户端. 设备接入和级联都会走数据库
	DeviceChannelsManager *DeviceChannels
)

type DeviceChannels struct {
	channels map[string][]*dao.ChannelModel
	lock     sync.RWMutex
}

func (d *DeviceChannels) AddChannel(deviceId string, channel *dao.ChannelModel) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.channels == nil {
		d.channels = make(map[string][]*dao.ChannelModel, 5)
	}

	channels := d.channels[deviceId]
	channels = append(channels, channel)
	d.channels[deviceId] = channels
}

func (d *DeviceChannels) FindChannels(deviceId string) []*dao.ChannelModel {
	d.lock.RLock()
	defer d.lock.RUnlock()

	return d.channels[deviceId]
}
