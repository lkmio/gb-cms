package main

import "strings"

// Handler 处理下级设备的消息
type Handler interface {
	OnUnregister(id string)

	OnRegister(id, transport, addr string) (int, GBDevice, bool)

	OnKeepAlive(id string) bool

	OnCatalog(device GBDevice, response *CatalogResponse)

	OnRecord(device GBDevice, response *QueryRecordInfoResponse)

	OnDeviceInfo(device GBDevice, response *DeviceInfoResponse)

	OnNotifyPosition(notify *MobilePositionNotify)
}

type EventHandler struct {
}

func (e *EventHandler) OnUnregister(id string) {
	device := DeviceManager.Find(id)
	if device != nil {
		device.(*Device).Status = OFF
	}

	if DB != nil {
		_ = DB.SaveDevice(device.(*Device))
	}
}

func (e *EventHandler) OnRegister(id, transport, addr string) (int, GBDevice, bool) {
	// 不能和级联设备的上级ID冲突
	if PlatformManager.FindPlatform(id) != nil {
		Sugar.Errorf("注册失败, ID与级联设备冲突. device: %s", id)
		return -1, nil, false
	}

	var device *Device
	old := DeviceManager.Find(id)

	if old != nil {
		old.(*Device).ID = id
		old.(*Device).Transport = transport
		old.(*Device).RemoteAddr = addr

		device = old.(*Device)
	} else {
		device = &Device{
			ID:         id,
			Transport:  transport,
			RemoteAddr: addr,
		}

		DeviceManager.Add(device)
	}

	device.Status = ON
	if DB != nil {
		if err := DB.SaveDevice(device); err != nil {
			Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", id, err.Error())
		}
	}

	return 3600, device, device.ChannelsTotal < 1
}

func (e *EventHandler) OnKeepAlive(id string) bool {
	device := DeviceManager.Find(id)
	if device == nil {
		Sugar.Errorf("更新心跳失败, 设备不存在. device: %s", id)
		return false
	}

	if !device.(*Device).Online() {
		Sugar.Errorf("更新心跳失败, 设备离线. device: %s", id)
	}

	if DB != nil {
		if err := DB.RefreshHeartbeat(id); err != nil {
			Sugar.Errorf("更新有效期失败. device: %s err: %s", id, err.Error())
		}
	}

	return true
}

func (e *EventHandler) OnCatalog(device GBDevice, response *CatalogResponse) {
	if DB == nil {
		return
	}

	id := device.GetID()
	for _, channel := range response.DeviceList.Devices {
		// 状态转为大写
		channel.Status = OnlineStatus(strings.ToUpper(channel.Status.String()))

		// 默认在线
		if OFF != channel.Status {
			channel.Status = ON
		}

		// 判断之前是否已经存在通道, 如果不存在累加总数
		old, _ := DB.QueryChannel(id, channel.DeviceID)

		if err := DB.SaveChannel(id, channel); err != nil {
			Sugar.Infof("保存通道到数据库失败 err: %s", err.Error())
		}

		if old == nil {
			device.(*Device).ChannelsTotal++
			device.(*Device).ChannelsOnline++
		} else if old.Status != channel.Status {
			// 保留处理其他状态
			if ON == channel.Status {
				device.(*Device).ChannelsOnline++
			} else if OFF == channel.Status {
				device.(*Device).ChannelsOnline--
			} else {
				return
			}
		}

		if err := DB.SaveDevice(device.(*Device)); err != nil {
			Sugar.Errorf("更新设备在线数失败 err: %s", err.Error())
		}
	}
}

func (e *EventHandler) OnRecord(device GBDevice, response *QueryRecordInfoResponse) {
	event := SNManager.FindEvent(response.SN)
	if event == nil {
		Sugar.Errorf("处理录像查询响应失败 SN: %d", response.SN)
		return
	}

	event(response)
}

func (e *EventHandler) OnDeviceInfo(device GBDevice, response *DeviceInfoResponse) {
	device.(*Device).Manufacturer = response.Manufacturer
	device.(*Device).Model = response.Model
	device.(*Device).Firmware = response.Firmware
	device.(*Device).Name = response.DeviceName

	if DB != nil {
		if err := DB.SaveDevice(device.(*Device)); err != nil {
			Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", device.GetID(), err.Error())
		}
	}
}

func (e *EventHandler) OnNotifyPosition(notify *MobilePositionNotify) {

}
