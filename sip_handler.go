package main

import (
	"github.com/lkmio/avformat/utils"
	"strconv"
	"strings"
	"time"
)

// Handler 处理下级设备的消息
type Handler interface {
	OnUnregister(id string)

	OnRegister(id, transport, addr string) (int, GBDevice, bool)

	OnKeepAlive(id string) bool

	OnCatalog(device string, response *CatalogResponse)

	OnRecord(device string, response *QueryRecordInfoResponse)

	OnDeviceInfo(device string, response *DeviceInfoResponse)

	OnNotifyPosition(notify *MobilePositionNotify)
}

type EventHandler struct {
}

func (e *EventHandler) OnUnregister(id string) {
	_ = DeviceDao.UpdateDeviceStatus(id, OFF)
}

func (e *EventHandler) OnRegister(id, transport, addr string) (int, GBDevice, bool) {
	now := time.Now()
	device := &Device{
		DeviceID:      id,
		Transport:     transport,
		RemoteAddr:    addr,
		Status:        ON,
		RegisterTime:  now,
		LastHeartbeat: now,
	}

	if err := DeviceDao.SaveDevice(device); err != nil {
		Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", id, err.Error())
	}

	count, _ := ChannelDao.QueryChanelCount(id)
	return 3600, device, count < 1
}

func (e *EventHandler) OnKeepAlive(id string, addr string) bool {
	now := time.Now()
	if err := DeviceDao.RefreshHeartbeat(id, now, addr); err != nil {
		Sugar.Errorf("更新有效期失败. device: %s err: %s", id, err.Error())
		return false
	}

	OnlineDeviceManager.Add(id, now)
	return true
}

func (e *EventHandler) OnCatalog(device string, response *CatalogResponse) {
	utils.Assert(device == response.DeviceID)
	for _, channel := range response.DeviceList.Devices {
		// 状态转为大写
		channel.Status = OnlineStatus(strings.ToUpper(channel.Status.String()))

		// 默认在线
		if OFF != channel.Status {
			channel.Status = ON
		}

		// 下级设备的系统ID, 更新DeviceInfo
		if channel.DeviceID == device && DeviceDao.ExistDevice(device) {
			_ = DeviceDao.UpdateDeviceInfo(device, &Device{
				Manufacturer: channel.Manufacturer,
				Model:        channel.Model,
				Name:         channel.Name,
			})
		}

		typeCode := GetTypeCode(channel.DeviceID)
		if typeCode == "" {
			Sugar.Errorf("保存通道时, 获取设备类型失败 device: %s", channel.DeviceID)
		}

		var groupId string
		if channel.ParentID != "" {
			layers := strings.Split(channel.ParentID, "/")
			groupId = layers[len(layers)-1]
		} else if channel.BusinessGroupID != "" {
			groupId = channel.BusinessGroupID
		}

		code, _ := strconv.Atoi(typeCode)
		channel.RootID = device
		channel.TypeCode = code
		channel.GroupID = groupId
		if err := ChannelDao.SaveChannel(channel); err != nil {
			Sugar.Infof("保存通道到数据库失败 err: %s", err.Error())
		}
	}
}

func GetTypeCode(id string) string {
	if len(id) != 20 {
		return ""
	}

	return id[10:13]
}

func (e *EventHandler) OnRecord(device string, response *QueryRecordInfoResponse) {
	event := SNManager.FindEvent(response.SN)
	if event == nil {
		Sugar.Errorf("处理录像查询响应失败 SN: %d", response.SN)
		return
	}

	event(response)
}

func (e *EventHandler) OnDeviceInfo(device string, response *DeviceInfoResponse) {
	if err := DeviceDao.UpdateDeviceInfo(device, &Device{
		Manufacturer: response.Manufacturer,
		Model:        response.Model,
		Firmware:     response.Firmware,
		Name:         response.DeviceName,
	}); err != nil {
		Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", device, err.Error())
	}
}

func (e *EventHandler) OnNotifyPosition(notify *MobilePositionNotify) {

}
