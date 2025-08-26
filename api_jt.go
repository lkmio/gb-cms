package main

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"math"
	"net/http"
)

func CheckJTDeviceOptions(device *dao.JTDeviceModel) error {
	if err := stack.CheckSipUAOptions(&common.SIPUAOptions{
		Username:   device.Username,
		ServerAddr: device.ServerAddr,
		ServerID:   device.SeverID,
	}); err != nil {
		return err
	} else if device.SimNumber == "" {
		// sim卡号必选项
		return fmt.Errorf("sim number is required")
	} else if dao.Device.ExistDevice(device.Username) {
		// 用户名与下级设备冲突
		return fmt.Errorf("username already exists")
	}

	return nil
}

func SaveAndStartJTDevice(device *dao.JTDeviceModel) error {
	jtDevice, err := stack.NewJTDevice(device, common.SipStack)
	if err != nil {
		log.Sugar.Errorf("create virtual device failed: %s", err.Error())
		return err
	}

	if !stack.JTDeviceManager.Add(device.Username, jtDevice) {
		return fmt.Errorf("ua添加失败, id冲突. key: %s", device.Username)
	}

	jtDevice.Start()
	return nil
}

func EqualJTDeviceOptions(old, new *dao.JTDeviceModel) bool {
	return stack.EqualSipUAOptions(&common.SIPUAOptions{
		Username:          old.Username,
		ServerAddr:        old.ServerAddr,
		Transport:         old.Transport,
		RegisterExpires:   old.RegisterExpires,
		Password:          old.Password,
		KeepaliveInterval: old.KeepaliveInterval,
		ServerID:          old.SeverID,
	}, &common.SIPUAOptions{
		Username:          new.Username,
		ServerAddr:        new.ServerAddr,
		Transport:         new.Transport,
		RegisterExpires:   new.RegisterExpires,
		Password:          new.Password,
		KeepaliveInterval: new.KeepaliveInterval,
		ServerID:          new.SeverID,
	})
}

func (api *ApiServer) OnVirtualDeviceAdd(device *dao.JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("add virtual device: %v", *device)

	err := CheckJTDeviceOptions(device)
	if err != nil {
		log.Sugar.Errorf("%s", err.Error())
		return nil, err
	} else if stack.JTDeviceManager.Find(device.Username) != nil {
		log.Sugar.Errorf("jt device already exists: %s", device.Username)
		return nil, fmt.Errorf("jt device already exists: %s", device.Username)
	}

	err = dao.JTDevice.SaveDevice(device)
	if err != nil {
		log.Sugar.Errorf("save device failed: %s", err.Error())
		return nil, err
	}

	err = SaveAndStartJTDevice(device)
	if err != nil {
		log.Sugar.Errorf("start device failed: %s", err.Error())
	}

	return nil, err
}

func (api *ApiServer) OnVirtualDeviceEdit(device *dao.JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("edit virtual device: %v", *device)

	err := CheckJTDeviceOptions(device)
	if err != nil {
		log.Sugar.Errorf("%s", err.Error())
		return nil, err
	}

	oldDevice, err := dao.JTDevice.QueryDeviceByID(device.ID)
	if err != nil {
		log.Sugar.Errorf("jt device not found: %d", device.ID)
		return nil, err
	}

	if err = dao.JTDevice.UpdateDevice(device); err != nil {
		log.Sugar.Errorf("update device failed: %s", err.Error())
		return nil, err
	}

	// 国标ID发生变更, 更新通道的RootID
	if oldDevice.Username != device.Username {
		_ = dao.Channel.UpdateRootID(oldDevice.Username, device.Username)
	}

	// sim卡号发生变更, 告知media server关闭推流, 关闭与上级的转发sink
	if oldDevice.SimNumber != device.SimNumber {
		log.Sugar.Infof("sim number changed, close streams")
		streams, _ := dao.Stream.DeleteStreamByDeviceID(oldDevice.SimNumber)
		for _, stream := range streams {
			(&stack.Stream{stream}).Close(true, true)
		}
	}

	// SipUA信息发生变更, 则需要重启设备
	if !EqualJTDeviceOptions(oldDevice, device) {
		log.Sugar.Infof("sipua options changed, restart device")
		// 重启设备
		if client := stack.JTDeviceManager.Remove(oldDevice.Username); client != nil {
			client.Stop()
		}

		err = SaveAndStartJTDevice(device)
		if err != nil {
			log.Sugar.Errorf("update device failed: %s", err.Error())
		}
	} else {
		log.Sugar.Infof("device info changed, update device info")
		if client := stack.JTDeviceManager.Find(oldDevice.Username); client != nil {
			client.SetDeviceInfo(device.Name, device.Manufacturer, device.Model, device.Firmware)
		}
	}

	return nil, err
}

func (api *ApiServer) OnVirtualDeviceRemove(device *dao.JTDeviceModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("remove virtual device: %v", *device)

	err := dao.JTDevice.DeleteDevice(device.Username)
	if err != nil {
		return nil, err
	} else if client := stack.JTDeviceManager.Remove(device.Username); client != nil {
		client.Stop()
	}

	return nil, nil
}

func (api *ApiServer) OnVirtualChannelAdd(channel *dao.ChannelModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("add virtual channel: %v", *channel)

	device, err := dao.JTDevice.QueryDevice(channel.RootID)
	if err != nil {
		log.Sugar.Errorf("query jt device failed: %s device: %s ", err.Error(), channel.RootID)
		return nil, err
	}

	if len(channel.DeviceID) != 20 {
		log.Sugar.Errorf("invalid channel id: %s", channel.DeviceID)
		return nil, fmt.Errorf("invalid channel id: %s", channel.DeviceID)
	}

	channel.ParentID = device.Username
	channel.RootID = device.Username
	channel.GroupID = device.Username
	err = dao.Channel.SaveJTChannel(channel)
	if err != nil {
		log.Sugar.Errorf("save channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualChannelEdit(channel *dao.ChannelModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("edit virtual channel: %v", *channel)

	oldChannel, err := dao.Channel.QueryChannelByID(channel.ID)
	if err != nil {
		log.Sugar.Errorf("query channel failed: %s", err.Error())
		return nil, err
	}

	if oldChannel.Name == channel.Name {
		// 目前只支持修改通道名称
		log.Sugar.Warnf("channel name not changed: %s", channel.Name)
		return nil, nil
	}

	err = dao.Channel.UpdateChannel(channel)
	if err != nil {
		log.Sugar.Errorf("update channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualChannelRemove(channel *dao.ChannelModel, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("remove virtual channel: %v", *channel)

	device, err := dao.JTDevice.QueryDevice(channel.RootID)
	if err != nil {
		log.Sugar.Errorf("query jt device failed: %s device: %s ", err.Error(), channel.RootID)
		return nil, err
	}

	err = dao.Channel.DeleteChannel(device.Username, channel.DeviceID)
	if err != nil {
		log.Sugar.Errorf("delete channel failed: %s", err.Error())
	}
	return nil, err
}

func (api *ApiServer) OnVirtualDeviceList(v *PageQuery, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("query virtual device list: %v", *v)

	if v.PageNumber == nil {
		var defaultPageNumber = 1
		v.PageNumber = &defaultPageNumber
	}

	if v.PageSize == nil {
		var defaultPageSize = 10
		v.PageSize = &defaultPageSize
	}

	devices, total, err := dao.JTDevice.QueryDevices(*v.PageNumber, *v.PageSize)
	if err != nil {
		log.Sugar.Errorf("查询设备列表失败 err: %s", err.Error())
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
