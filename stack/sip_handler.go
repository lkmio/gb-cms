package stack

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
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
	_ = dao.Device.UpdateDeviceStatus(id, common.OFF)
}

func (e *EventHandler) OnRegister(id, transport, addr string) (int, GBDevice, bool) {
	now := time.Now()
	device := &dao.DeviceModel{
		DeviceID:      id,
		Transport:     transport,
		RemoteAddr:    addr,
		Status:        common.ON,
		RegisterTime:  now,
		LastHeartbeat: now,
	}

	if err := dao.Device.SaveDevice(device); err != nil {
		log.Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", id, err.Error())
	}

	count, _ := dao.Channel.QueryChanelCount(id)
	return 3600, &Device{device}, count < 1
}

func (e *EventHandler) OnKeepAlive(id string, addr string) bool {
	now := time.Now()
	if err := dao.Device.RefreshHeartbeat(id, now, addr); err != nil {
		log.Sugar.Errorf("更新有效期失败. device: %s err: %s", id, err.Error())
		return false
	}

	OnlineDeviceManager.Add(id, now)
	return true
}

func (e *EventHandler) OnCatalog(device string, response *CatalogResponse) {
	utils.Assert(device == response.DeviceID)
	for _, channel := range response.DeviceList.Devices {
		// 状态转为大写
		channel.Status = common.OnlineStatus(strings.ToUpper(channel.Status.String()))

		// 默认在线
		if common.OFF != channel.Status {
			channel.Status = common.ON
		}

		// 下级设备的系统ID, 更新DeviceInfo
		if channel.DeviceID == device && dao.Device.ExistDevice(device) {
			_ = dao.Device.UpdateDeviceInfo(device, &dao.DeviceModel{
				Manufacturer: channel.Manufacturer,
				Model:        channel.Model,
				Name:         channel.Name,
			})
		}

		typeCode := GetTypeCode(channel.DeviceID)
		if typeCode == "" {
			log.Sugar.Errorf("保存通道时, 获取设备类型失败 device: %s", channel.DeviceID)
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
		if err := dao.Channel.SaveChannel(channel); err != nil {
			log.Sugar.Infof("保存通道到数据库失败 err: %s", err.Error())
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
		log.Sugar.Errorf("处理录像查询响应失败 SN: %d", response.SN)
		return
	}

	event(response)
}

func (e *EventHandler) OnDeviceInfo(device string, response *DeviceInfoResponse) {
	if err := dao.Device.UpdateDeviceInfo(device, &dao.DeviceModel{
		Manufacturer: response.Manufacturer,
		Model:        response.Model,
		Firmware:     response.Firmware,
		Name:         response.DeviceName,
	}); err != nil {
		log.Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", device, err.Error())
	}
}

func (e *EventHandler) OnNotifyPosition(notify *MobilePositionNotify) {

}
