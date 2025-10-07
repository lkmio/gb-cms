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

	OnNotifyCatalog(catalog *CatalogResponse)
}

type EventHandler struct {
}

func (e *EventHandler) OnUnregister(id string) {
	_ = dao.Device.UpdateDeviceStatus(id, common.OFF)
}

// OnRegister 处理设备注册请求
//
//	int - 注册有效期(秒)
//	GBDevice - 注册成功后返回的设备信息, 返回nil表示注册失败
//	bool - 是否需要发送目录查询(true表示需要)
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
		return 0, nil, false
	} else if d, err := dao.Device.QueryDevice(id); err == nil {
		// 查询所有字段
		device = d
	}

	OnlineDeviceManager.Add(id, now)
	count, _ := dao.Channel.QueryChanelCount(id, true)
	return 3600, &Device{device}, count < 1 || dao.Device.QueryNeedRefreshCatalog(id, now)
}

func (e *EventHandler) OnKeepAlive(id string, addr string) bool {
	now := time.Now()
	if !OnlineDeviceManager.Refresh(id, now) {
		// 拒绝设备离线后收到的心跳, 让设备重新发起注册
		return false
	} else if err := dao.Device.RefreshHeartbeat(id, now, addr); err != nil {
		log.Sugar.Errorf("更新有效期失败. device: %s err: %s", id, err.Error())
		return false
	}

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

func (e *EventHandler) SavePosition(position *dao.PositionModel) {
	// 更新设备最新的位置
	if position.DeviceID == position.ChannelID || position.ChannelID == "" {
		conditions := make(map[string]interface{}, 0)
		conditions["longitude"] = position.Longitude
		conditions["latitude"] = position.Latitude
		_ = dao.Device.UpdateDevice(position.DeviceID, conditions)
	}

	if common.Config.PositionReserveDays < 1 {
		return
	}

	if err := dao.Position.SavePosition(position); err != nil {
		log.Sugar.Errorf("保存位置信息到数据库失败 device: %s err: %s", position.DeviceID, err.Error())
	}
}

func (e *EventHandler) OnNotifyPositionMessage(notify *MobilePositionNotify) {
	model := dao.PositionModel{
		DeviceID:  notify.DeviceID,
		Longitude: notify.Longitude,
		Latitude:  notify.Latitude,
		Speed:     notify.Speed,
		Direction: notify.Direction,
		Altitude:  notify.Altitude,
		Time:      notify.Time,
		Source:    dao.PositionSourceSubscribe,
	}

	e.SavePosition(&model)
}

func (e *EventHandler) OnNotifyCatalogMessage(catalog *CatalogResponse) {
	for _, channel := range catalog.DeviceList.Devices {
		if channel.Event == "" {
			log.Sugar.Warnf("目录事件为空 设备ID: %s", channel.DeviceID)
			continue
		}

		channel.RootID = catalog.DeviceID
		switch channel.Event {
		case "ON":
			_ = dao.Channel.UpdateChannelStatus(catalog.DeviceID, channel.DeviceID, string(common.ON))
			break
		case "OFF":
			_ = dao.Channel.UpdateChannelStatus(catalog.DeviceID, channel.DeviceID, string(common.OFF))
			break
		case "VLOST":
			break
		case "DEFECT":
			break
		case "ADD":
			_ = dao.Channel.SaveChannel(channel)
			break
		case "DEL":
			_ = dao.Channel.DeleteChannel(catalog.DeviceID, channel.DeviceID)
			break
		case "UPDATE":
			_ = dao.Channel.SaveChannel(channel)
			break
		default:
			log.Sugar.Warnf("未知的目录事件 %s 设备ID: %s", channel.Event, channel.DeviceID)
		}
	}
}

func (e *EventHandler) OnNotifyAlarmMessage(deviceId string, alarm *AlarmNotify) {
	// 保存报警携带的位置信息
	if alarm.Longitude != nil && alarm.Latitude != nil {
		e.SavePosition(&dao.PositionModel{
			DeviceID:  deviceId,
			ChannelID: alarm.DeviceID,
			Longitude: *alarm.Longitude,
			Latitude:  *alarm.Latitude,
			Time:      alarm.AlarmTime,
			Source:    dao.PositionSourceAlarm,
		})
	}

	if common.Config.AlarmReserveDays < 1 {
		return
	}

	model := dao.AlarmModel{
		DeviceID:      deviceId,
		ChannelID:     alarm.DeviceID,
		AlarmPriority: alarm.AlarmPriority,
		AlarmMethod:   alarm.AlarmMethod,
		Time:          alarm.AlarmTime,
		Description:   alarm.AlarmDescription,
		Longitude:     alarm.Longitude,
		Latitude:      alarm.Latitude,
	}

	if alarm.Info != nil {
		model.AlarmType = alarm.Info.AlarmType
		if alarm.Info.AlarmTypeParam != nil {
			model.EventType = alarm.Info.AlarmTypeParam.EventType
		}
	}

	if err := dao.Alarm.Save(&model); err != nil {
		log.Sugar.Errorf("保存报警信息到数据库失败 device: %s err: %s", alarm.DeviceID, err.Error())
	}
}
