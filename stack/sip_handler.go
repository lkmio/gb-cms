package stack

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/lkmio/avformat/utils"
	"net"
	"strconv"
	"time"
)

// Handler 处理下级设备的消息
type Handler interface {
	OnUnregister(id string)

	OnRegister(id, transport, addr string, userAgent string) (int, GBDevice, bool)

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

func (e *EventHandler) OnRegister(id, transport, addr, userAgent string) (int, GBDevice, bool) {
	now := time.Now()
	host, p, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(p)
	device := &dao.DeviceModel{
		DeviceID:      id,
		Transport:     transport,
		RemoteIP:      host,
		RemotePort:    port,
		UserAgent:     userAgent,
		Status:        common.ON,
		RegisterTime:  now,
		LastHeartbeat: now,
	}

	if err := dao.Device.SaveDevice(device); err != nil {
		log.Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", id, err.Error())
	}

	count, _ := dao.Channel.QueryChanelCount(id, true)
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
	if event := SNManager.FindEvent(response.SN); event != nil {
		event(response)
	} else {
		log.Sugar.Errorf("处理目录响应失败 SN: %d", response.SN)
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
