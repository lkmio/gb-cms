package main

import (
	"fmt"
	"math"
	"net/http"
)

func CheckJTDeviceOptions(device *JTDeviceModel) error {
	if err := CheckSipUAOptions(&SIPUAOptions{
		Username:   device.Username,
		ServerAddr: device.ServerAddr,
		ServerID:   device.SeverID,
	}); err != nil {
		return err
	} else if device.SimNumber == "" {
		// sim卡号必选项
		return fmt.Errorf("sim number is required")
	} else if DeviceDao.ExistDevice(device.Username) {
		// 用户名与下级设备冲突
		return fmt.Errorf("username already exists")
	}

	return nil
}

func SaveAndStartJTDevice(device *JTDeviceModel) error {
	jtDevice, err := NewJTDevice(device, SipStack)
	if err != nil {
		Sugar.Errorf("create virtual device failed: %s", err.Error())
		return err
	}

	if !JTDeviceManager.Add(device.Username, jtDevice) {
		return fmt.Errorf("ua添加失败, id冲突. key: %s", device.Username)
	}

	jtDevice.Start()
	return nil
}

func EqualJTDeviceOptions(old, new *JTDeviceModel) bool {
	return EqualSipUAOptions(&SIPUAOptions{
		Username:          old.Username,
		ServerAddr:        old.ServerAddr,
		Transport:         old.Transport,
		RegisterExpires:   old.RegisterExpires,
		Password:          old.Password,
		KeepaliveInterval: old.KeepaliveInterval,
		ServerID:          old.SeverID,
	}, &SIPUAOptions{
		Username:          new.Username,
		ServerAddr:        new.ServerAddr,
		Transport:         new.Transport,
		RegisterExpires:   new.RegisterExpires,
		Password:          new.Password,
		KeepaliveInterval: new.KeepaliveInterval,
		ServerID:          new.SeverID,
	})
}

func (api *ApiServer) OnVirtualDeviceAdd(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("add virtual device: %v", *device)

	err := CheckJTDeviceOptions(device)
	if err != nil {
		Sugar.Errorf("%s", err.Error())
		return nil, err
	} else if JTDeviceManager.Find(device.Username) != nil {
		Sugar.Errorf("jt device already exists: %s", device.Username)
		return nil, fmt.Errorf("jt device already exists: %s", device.Username)
	}

	err = JTDeviceDao.SaveDevice(device)
	if err != nil {
		Sugar.Errorf("save device failed: %s", err.Error())
		return nil, err
	}

	err = SaveAndStartJTDevice(device)
	if err != nil {
		Sugar.Errorf("start device failed: %s", err.Error())
	}

	return nil, err
}

func (api *ApiServer) OnVirtualDeviceEdit(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("edit virtual device: %v", *device)

	err := CheckJTDeviceOptions(device)
	if err != nil {
		Sugar.Errorf("%s", err.Error())
		return nil, err
	}

	oldDevice, err := JTDeviceDao.QueryDeviceByID(device.ID)
	if err != nil {
		Sugar.Errorf("jt device not found: %d", device.ID)
		return nil, err
	}

	if err = JTDeviceDao.UpdateDevice(device); err != nil {
		Sugar.Errorf("update device failed: %s", err.Error())
		return nil, err
	}

	// 国标ID发生变更, 更新通道的RootID
	if oldDevice.Username != device.Username {
		_ = ChannelDao.UpdateRootID(oldDevice.Username, device.Username)
	}

	// sim卡号发生变更, 告知media server关闭推流, 关闭与上级的转发sink
	if oldDevice.SimNumber != device.SimNumber {
		Sugar.Infof("sim number changed, close streams")
		streams, _ := StreamDao.DeleteStreamByDeviceID(oldDevice.SimNumber)
		for _, stream := range streams {
			stream.Close(true, true)
		}
	}

	// SipUA信息发生变更, 则需要重启设备
	if !EqualJTDeviceOptions(oldDevice, device) {
		Sugar.Infof("sipua options changed, restart device")
		// 重启设备
		if client := JTDeviceManager.Remove(oldDevice.Username); client != nil {
			client.Stop()
		}

		err = SaveAndStartJTDevice(device)
		if err != nil {
			Sugar.Errorf("update device failed: %s", err.Error())
		}
	} else {
		Sugar.Infof("device info changed, update device info")
		if client := JTDeviceManager.Find(oldDevice.Username); client != nil {
			client.SetDeviceInfo(device.Name, device.Manufacturer, device.Model, device.Firmware)
		}
	}

	return nil, err
}

func (api *ApiServer) OnVirtualDeviceRemove(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("remove virtual device: %v", *device)

	err := JTDeviceDao.DeleteDevice(device.Username)
	if err != nil {
		return nil, err
	} else if client := JTDeviceManager.Remove(device.Username); client != nil {
		client.Stop()
	}

	return nil, nil
}

func (api *ApiServer) OnVirtualChannelAdd(channel *Channel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("add virtual channel: %v", *channel)

	device, err := JTDeviceDao.QueryDevice(channel.RootID)
	if err != nil {
		Sugar.Errorf("query jt device failed: %s device: %s ", err.Error(), channel.RootID)
		return nil, err
	}

	if len(channel.DeviceID) != 20 {
		Sugar.Errorf("invalid channel id: %s", channel.DeviceID)
		return nil, fmt.Errorf("invalid channel id: %s", channel.DeviceID)
	}

	channel.ParentID = device.Username
	channel.RootID = device.Username
	channel.GroupID = device.Username
	err = ChannelDao.SaveJTChannel(channel)
	if err != nil {
		Sugar.Errorf("save channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualChannelEdit(channel *Channel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("edit virtual channel: %v", *channel)

	oldChannel, err := ChannelDao.QueryChannelByID(channel.ID)
	if err != nil {
		Sugar.Errorf("query channel failed: %s", err.Error())
		return nil, err
	}

	if oldChannel.Name == channel.Name {
		// 目前只支持修改通道名称
		Sugar.Warnf("channel name not changed: %s", channel.Name)
		return nil, nil
	}

	err = ChannelDao.UpdateChannel(channel)
	if err != nil {
		Sugar.Errorf("update channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualChannelRemove(channel *Channel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("remove virtual channel: %v", *channel)

	device, err := JTDeviceDao.QueryDevice(channel.RootID)
	if err != nil {
		Sugar.Errorf("query jt device failed: %s device: %s ", err.Error(), channel.RootID)
		return nil, err
	}

	err = ChannelDao.DeleteChannel(device.Username, channel.DeviceID)
	if err != nil {
		Sugar.Errorf("delete channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualDeviceList(v *PageQuery, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("query virtual device list: %v", *v)

	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	devices, total, err := JTDeviceDao.QueryDevices(*v.PageNumber, *v.PageSize)
	if err != nil {
		Sugar.Errorf("查询设备列表失败 err: %s", err.Error())
		return nil, err
	}

	query := &PageQuery{
		PageNumber: v.PageNumber,
		PageSize:   v.PageSize,
		TotalCount: total,
		TotalPages: int(math.Ceil(float64(total) / float64(*v.PageSize))),
		Data:       devices,
	}

	return query, nil
}
