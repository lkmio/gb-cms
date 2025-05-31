package main

import (
	"fmt"
	"net/http"
)

func (api *ApiServer) OnVirtualDeviceAdd(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	Sugar.Infof("add virtual device: %v", *device)

	if len(device.Username) != 20 {
		Sugar.Errorf("invalid username: %s", device.Username)
		return nil, fmt.Errorf("invalid username: %s", device.Username)
	} else if len(device.SeverID) != 20 {
		Sugar.Errorf("invalid server id: %s", device.SeverID)
		return nil, fmt.Errorf("invalid server id: %s", device.SeverID)
	} else if device.SimNumber == "" {
		// sim卡号必选项
		Sugar.Errorf("sim number is required")
		return nil, fmt.Errorf("sim number is required")
	}

	if JTDeviceDao.ExistDevice(device.Username, device.SimNumber) {
		// 用户名或sim卡号已存在
		Sugar.Errorf("username or sim number already exists")
		return nil, fmt.Errorf("username or sim number already exists")
	} else if DeviceDao.ExistDevice(device.Username) {
		// 用户名与下级设备冲突
		Sugar.Errorf("username already exists")
		return nil, fmt.Errorf("username already exists")
	}

	jtDevice, err := NewJTDevice(device, SipStack)
	if err != nil {
		Sugar.Errorf("create virtual device failed: %s", err.Error())
		return nil, err
	}

	if !JTDeviceManager.Add(device.Username, jtDevice) {
		return nil, fmt.Errorf("ua添加失败, id冲突. key: %s", device.Username)
	} else if err = JTDeviceDao.SaveDevice(device); err != nil {
		JTDeviceManager.Remove(device.Username)
		Sugar.Errorf("save device failed: %s", err.Error())
		return nil, err
	}

	jtDevice.Start()

	if err != nil {
		Sugar.Errorf("add jt device failed: %s", err.Error())
		return nil, err
	}

	return nil, nil
}

func (api *ApiServer) OnVirtualDeviceEdit(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {

	return nil, nil
}

func (api *ApiServer) OnVirtualDeviceRemove(device *JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

	return nil, nil
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
