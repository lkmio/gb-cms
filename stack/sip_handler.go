package stack

import (
	"encoding/xml"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
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
	CloseDevice(id)
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
	model := &dao.DeviceModel{
		DeviceID:      id,
		Transport:     transport,
		RemoteIP:      host,
		RemotePort:    port,
		UserAgent:     userAgent,
		Status:        common.ON,
		RegisterTime:  now,
		LastHeartbeat: now,
	}

	if err := dao.Device.SaveDevice(model); err != nil {
		log.Sugar.Errorf("保存设备信息到数据库失败 device: %s err: %s", id, err.Error())
		return 0, nil, false
	} else if d, err := dao.Device.QueryDevice(id); err == nil {
		// 查询所有字段
		model = d
	}

	_, alreadyOnline := OnlineDeviceManager.Find(id)
	OnlineDeviceManager.Add(id, now)
	count, _ := dao.Channel.QueryChanelCount(id, true)

	// 级联通知通道上线
	device := &Device{model}
	if count > 0 && !alreadyOnline {
		go device.PushCatalog()
	}

	return 3600, device, count < 1 || dao.Device.QueryNeedRefreshCatalog(id, now)
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

	// 不保留位置信令
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

// ForwardCatalogNotifyMessage 转发目录变化到级联上级
func ForwardCatalogNotifyMessage(catalog *CatalogResponse) {
	for _, channel := range catalog.DeviceList.Devices {
		// 跳过没有级联的通道
		platforms := FindChannelSharedPlatforms(catalog.DeviceID, channel.DeviceID)
		if len(platforms) < 1 {
			continue
		}

		for _, platform := range platforms {
			// 跳过离线的级联设备
			if !platform.Online() {
				continue
			}

			// 先查出customid
			model, _ := dao.Channel.QueryChannel(catalog.DeviceID, channel.DeviceID)

			newCatalog := *catalog
			newCatalog.SumNum = 1
			newCatalog.DeviceID = platform.GetID()
			newCatalog.DeviceList.Devices = []*dao.ChannelModel{channel}
			newCatalog.DeviceList.Num = 1

			// 优先使用自定义ID
			if model != nil && model.CustomID != nil && *model.CustomID != channel.DeviceID {
				newCatalog.DeviceList.Devices[0].DeviceID = *model.CustomID
			}

			// 格式化消息
			indent, err := xml.MarshalIndent(&newCatalog, " ", "")
			if err != nil {
				log.Sugar.Errorf("报警消息格式化失败 err: %s", err.Error())
				continue
			}

			// 已经订阅走订阅
			request, err := platform.CreateRequestByDialogType(dao.SipDialogTypeSubscribeCatalog, sip.NOTIFY)
			if err == nil {
				body := AddXMLHeader(string(indent))
				request.SetBody(body, true)
				_ = platform.SendRequest(request)
				continue
			}

			// 不基于会话创建一个notify消息
			request, err = platform.BuildRequest(sip.NOTIFY, &XmlMessageType, string(indent))
			if err != nil {
				log.Sugar.Errorf("创建报警消息失败 err: %s", err.Error())
				continue
			}

			_ = platform.SendRequest(request)
		}
	}
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

	ForwardCatalogNotifyMessage(catalog)
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

	channel, err := dao.Channel.QueryChannel(deviceId, alarm.DeviceID)
	if channel == nil {
		log.Sugar.Errorf("查询通道失败 err: %s device: %s channel: %s", err.Error(), deviceId, alarm.DeviceID)
		return
	}

	// 转发报警到级联上级
	platforms := FindChannelSharedPlatforms(deviceId, alarm.DeviceID)
	if len(platforms) < 1 {
		return
	}

	newAlarmMessage := *alarm
	// 优先使用自定义ID
	if channel.CustomID != nil && *channel.CustomID != alarm.DeviceID {
		newAlarmMessage.DeviceID = *channel.CustomID
	}

	for _, platform := range platforms {
		if !platform.Online() {
			continue
		}

		// 已经订阅走订阅, 没有订阅走报警事件通知
		request, err := platform.CreateRequestByDialogType(dao.SipDialogTypeSubscribeAlarm, sip.NOTIFY)
		if err == nil {
			// 格式化报警消息
			indent, err := xml.MarshalIndent(&newAlarmMessage, " ", "")
			if err != nil {
				log.Sugar.Errorf("报警消息格式化失败 err: %s", err.Error())
				continue
			}

			body := AddXMLHeader(string(indent))
			request.SetBody(body, true)
			_ = platform.SendRequest(request)
			continue
		}

		platform.SendMessage(&newAlarmMessage)
	}
}
