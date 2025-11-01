package api

import (
	"context"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (api *ApiServer) OnDeviceList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	// 分页参数
	if q.Limit < 1 {
		q.Limit = 10
	}

	var status string
	if "" == q.Online {
	} else if "true" == q.Online {
		status = "ON"
	} else if "false" == q.Online {
		status = "OFF"
	}

	if "desc" != q.Order {
		q.Order = "asc"
	}

	devices, total, err := dao.Device.QueryDevices((q.Start/q.Limit)+1, q.Limit, status, q.Keyword, q.Order)
	if err != nil {
		log.Sugar.Errorf("查询设备列表失败 err: %s", err.Error())
		return nil, err
	}

	response := struct {
		DeviceCount  int
		DeviceList_  []LiveGBSDevice `json:"DeviceList"`
		DeviceRegion bool
	}{
		DeviceCount:  total,
		DeviceRegion: common.Config.IP2RegionEnable,
	}

	// livgbs设备离线后的最后心跳时间, 涉及到是否显示非法设备的批量删除按钮
	offlineTime := time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05")
	for _, device := range devices {
		// 更新正在查询通道的进度
		var catalogProgress string
		data := stack.UniqueTaskManager.Find(stack.GenerateCatalogTaskID(device.GetID()))
		if data != nil {
			catalogSize := data.(*stack.CatalogProgress)

			if catalogSize.TotalSize > 0 {
				catalogProgress = fmt.Sprintf("%d/%d", catalogSize.RecvSize, catalogSize.TotalSize)
			}
		}

		var lastKeealiveTime string
		if device.Online() {
			lastKeealiveTime = device.LastHeartbeat.Format("2006-01-02 15:04:05")
		} else {
			lastKeealiveTime = offlineTime
		}

		if device.CatalogInterval < 1 {
			device.CatalogInterval = dao.DefaultCatalogInterval
		}

		response.DeviceList_ = append(response.DeviceList_, LiveGBSDevice{
			AlarmSubscribe:   device.AlarmSubscribe,  // 报警订阅
			CatalogInterval:  device.CatalogInterval, // 目录刷新时间
			CatalogProgress:  catalogProgress,
			CatalogSubscribe: device.CatalogSubscribe, // 目录订阅
			ChannelCount:     device.ChannelsTotal,
			ChannelOverLoad:  false,
			Charset:          "GB2312",
			CivilCodeFirst:   false,
			CommandTransport: device.Transport,
			ContactIP:        "",
			CreatedAt:        device.CreatedAt.Format("2006-01-02 15:04:05"),
			CustomName:       "",
			DropChannelType:  device.DropChannelType,
			GBVer:            "",
			ID:               device.GetID(),
			KeepOriginalTree: false,
			LastKeepaliveAt:  lastKeealiveTime,
			LastRegisterAt:   device.RegisterTime.Format("2006-01-02 15:04:05"),
			Latitude:         device.Latitude,
			Longitude:        device.Longitude,
			//Manufacturer:       device.Manufacturer,
			Manufacturer:       device.UserAgent,
			MediaTransport:     device.GetSetup().Transport(),
			MediaTransportMode: device.GetSetup().String(),
			Name:               device.Name,
			Online:             device.Online(),
			PTZSubscribe:       false, // PTZ订阅2022
			Password:           "",
			PositionSubscribe:  device.PositionSubscribe, // 位置订阅
			RecordCenter:       false,
			RecordIndistinct:   false,
			RecvStreamIP:       "",
			RemoteIP:           device.RemoteIP,
			RemotePort:         device.RemotePort,
			RemoteRegion:       device.RemoteRegion,
			SMSGroupID:         "",
			SMSID:              "",
			StreamMode:         "",
			SubscribeInterval:  0,
			Type:               "GB",
			UpdatedAt:          device.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &response, nil
}

func (api *ApiServer) OnChannelList(q *QueryDeviceChannel, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	// 分页参数
	if q.Limit < 1 {
		q.Limit = 10
	}

	var deviceName string
	if q.DeviceID != "" {
		device, err := dao.Device.QueryDevice(q.DeviceID)
		if err != nil {
			log.Sugar.Errorf("查询设备失败 err: %s", err.Error())
			return nil, err
		}

		deviceName = device.Name
	}

	var status string
	if "" == q.Online {
	} else if "true" == q.Online {
		status = "ON"
	} else if "false" == q.Online {
		status = "OFF"
	}

	if "desc" != q.Order {
		q.Order = "asc"
	}

	channels, total, err := dao.Channel.QueryChannels(q.DeviceID, q.GroupID, (q.Start/q.Limit)+1, q.Limit, status, q.Keyword, q.Order, q.Sort, q.ChannelType == "dir")
	if err != nil {
		log.Sugar.Errorf("查询通道列表失败 err: %s", err.Error())
		return nil, err
	}

	response := ChannelListResult{
		ChannelCount: total,
	}

	index := q.Start + 1
	response.ChannelList = ChannelModels2LiveGBSChannels(index, channels, deviceName)
	return &response, nil
}

func (api *ApiServer) OnRecordList(v *QueryRecordParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("查询录像列表 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("查询录像列表失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	device := &stack.Device{DeviceModel: model}
	sn := stack.GetSN()
	err := device.QueryRecord(v.ChannelID, v.StartTime, v.EndTime, sn, "all")
	if err != nil {
		log.Sugar.Errorf("发送查询录像请求失败 err: %s", err.Error())
		return nil, err
	}

	// 设置查询超时时长
	timeout := int(math.Max(math.Min(5, float64(v.Timeout)), 60))
	withTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

	var recordList []stack.RecordInfo
	stack.SNManager.AddEvent(sn, func(data interface{}) {
		response := data.(*stack.QueryRecordInfoResponse)

		if len(response.DeviceList.Devices) > 0 {
			recordList = append(recordList, response.DeviceList.Devices...)
		}

		// 所有记录响应完毕
		if len(recordList) >= response.SumNum {
			cancelFunc()
		}
	})

	select {
	case _ = <-withTimeout.Done():
		break
	}

	response := struct {
		DeviceID   string
		Name       string
		RecordList []struct {
			DeviceID  string
			EndTime   string
			FileSize  uint64
			Name      string
			Secrecy   string
			StartTime string
			Type      string
		}
		SumNum int `json:"sumNum"`
	}{
		DeviceID: v.DeviceID,
		Name:     model.Name,
		SumNum:   len(recordList),
	}

	for _, record := range recordList {
		log.Sugar.Infof("查询录像列表 %v", record)
		response.RecordList = append(response.RecordList, struct {
			DeviceID  string
			EndTime   string
			FileSize  uint64
			Name      string
			Secrecy   string
			StartTime string
			Type      string
		}{
			DeviceID:  record.DeviceID,
			EndTime:   record.EndTime,
			FileSize:  record.FileSize,
			Name:      record.Name,
			Secrecy:   record.Secrecy,
			StartTime: record.StartTime,
			Type:      record.Type,
		})
	}

	return &response, nil
}

func (api *ApiServer) OnDeviceMediaTransportSet(req *SetMediaTransportReq, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	var setupType common.SetupType
	if "udp" == strings.ToLower(req.MediaTransport) {
		setupType = common.SetupTypeUDP
	} else if "passive" == strings.ToLower(req.MediaTransportMode) {
		setupType = common.SetupTypePassive
	} else if "active" == strings.ToLower(req.MediaTransportMode) {
		setupType = common.SetupTypeActive
	} else {
		return nil, fmt.Errorf("media_transport_mode error")
	}

	err := dao.Device.UpdateMediaTransport(req.DeviceID, setupType)
	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCatalogQuery(params *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	deviceModel, err := dao.Device.QueryDevice(params.DeviceID)
	if err != nil {
		return nil, err
	}

	if deviceModel == nil {
		return nil, fmt.Errorf("not found device")
	}

	list, err := (&stack.Device{DeviceModel: deviceModel}).QueryCatalog(15)
	if err != nil {
		return nil, err
	}

	response := struct {
		ChannelCount int                 `json:"ChannelCount"`
		ChannelList  []*dao.ChannelModel `json:"ChannelList"`
	}{
		ChannelCount: len(list),
		ChannelList:  list,
	}
	return &response, nil
}

func (api *ApiServer) OnDeviceTree(q *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	var response []*LiveGBSDeviceTree

	// 查询所有设备
	if q.DeviceID == "" && q.PCode == "" {
		devices, err := dao.Device.LoadDevices()
		if err != nil {
			return nil, err
		}

		for _, model := range devices {
			count, _ := dao.Channel.QueryChanelCount(model.DeviceID, true)
			deviceCount, _ := dao.Channel.QueryChanelCount(model.DeviceID, false)
			onlineCount, _ := dao.Channel.QueryOnlineChanelCount(model.DeviceID, false)
			response = append(response, &LiveGBSDeviceTree{Code: "", Custom: false, CustomID: "", CustomName: "", ID: model.DeviceID, Latitude: 0, Longitude: 0, Manufacturer: model.Manufacturer, Name: model.Name, OnlineSubCount: onlineCount, Parental: true, PtzType: 0, Serial: model.DeviceID, Status: model.Status.String(), SubCount: count, SubCountDevice: deviceCount})
		}
	} else {
		// 查询设备下的某个目录的所有通道
		if q.PCode == "" {
			q.PCode = q.DeviceID
		}
		channels, _, _ := dao.Channel.QueryChannels(q.DeviceID, q.PCode, -1, 0, "", "", "asc", "", false)
		for _, channel := range channels {
			id := channel.RootID + ":" + channel.DeviceID
			latitude, _ := strconv.ParseFloat(channel.Latitude, 10)
			longitude, _ := strconv.ParseFloat(channel.Longitude, 10)

			var deviceCount int
			var onlineCount int
			if channel.SubCount > 0 {
				deviceCount, _ = dao.Channel.QuerySubChannelCount(channel.RootID, channel.DeviceID, false)
				onlineCount, _ = dao.Channel.QueryOnlineSubChannelCount(channel.RootID, channel.DeviceID, false)
			}

			response = append(response, &LiveGBSDeviceTree{Code: channel.DeviceID, Custom: false, CustomID: "", CustomName: "", ID: id, Latitude: latitude, Longitude: longitude, Manufacturer: channel.Manufacturer, Name: channel.Name, OnlineSubCount: onlineCount, Parental: false, PtzType: 0, Serial: channel.RootID, Status: channel.Status.String(), SubCount: channel.SubCount, SubCountDevice: deviceCount})
		}
	}

	return &response, nil
}

func (api *ApiServer) OnDeviceRemove(q *DeleteDevice, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	var err error
	if q.IP != "" {
		// 删除IP下的所有设备
		err = dao.Device.DeleteDevicesByIP(q.IP)
	} else if q.UA != "" {
		//  删除UA下的所有设备
		err = dao.Device.DeleteDevicesByUA(q.UA)
	} else {
		// 删除单个设备
		err = dao.Device.DeleteDevice(q.DeviceID)
	}

	if err != nil {
		return nil, err
	} else if q.Forbid {
		if q.IP != "" {
			// 拉黑IP
			err = dao.Blacklist.SaveIP(q.IP)
		} else if q.UA != "" {
			// 拉黑UA
			err = dao.Blacklist.SaveUA(q.UA)
		}
	}

	return "OK", nil
}

func (api *ApiServer) OnDeviceInfoSet(params *DeviceInfo, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	model, err := dao.Device.QueryDevice(params.DeviceID)
	if err != nil {
		return nil, err
	}

	device := stack.Device{DeviceModel: model}

	// 更新的字段和值
	conditions := make(map[string]interface{}, 0)

	// 刷新目录间隔
	if params.CatalogInterval != model.CatalogInterval && params.CatalogInterval >= 60 {
		conditions["catalog_interval"] = params.CatalogInterval
	}

	if model.CatalogSubscribe != params.CatalogSubscribe {
		conditions["catalog_subscribe"] = params.CatalogSubscribe
		// 开启目录订阅
		if params.CatalogSubscribe {
			_ = device.SubscribeCatalog()
		} else {
			// 取消目录订阅
			device.UnsubscribeCatalog()
		}
	}

	if model.PositionSubscribe != params.PositionSubscribe {
		conditions["position_subscribe"] = params.PositionSubscribe
		// 开启位置订阅
		if params.PositionSubscribe {
			_ = device.SubscribePosition()
		} else {
			// 取消位置订阅
			device.UnsubscribePosition()
		}
	}

	if model.AlarmSubscribe != params.AlarmSubscribe {
		conditions["alarm_subscribe"] = params.AlarmSubscribe
		// 开启报警订阅
		if params.AlarmSubscribe {
			_ = device.SubscribeAlarm()
		} else {
			// 取消报警订阅
			device.UnsubscribeAlarm()
		}
	}

	// 更新设备信息
	if len(conditions) > 0 {
		if err = dao.Device.UpdateDevice(params.DeviceID, conditions); err != nil {
			return nil, err
		}
	}

	if params.DropChannelType != model.DropChannelType {
		var dropChannelTypes []string
		if params.DropChannelType != "" {
			dropChannelTypes = strings.Split(params.DropChannelType, ",")
		}
		if err = dao.Device.SetDropChannelType(params.DeviceID, dropChannelTypes); err != nil {
			return nil, err
		}
	}

	return "OK", nil
}

func (api *ApiServer) OnPTZControl(v *QueryRecordParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("PTZ控制 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		log.Sugar.Errorf("PTZ控制失败, 设备离线 device: %s", v.DeviceID)
		return nil, fmt.Errorf("设备离线")
	}

	device := &stack.Device{DeviceModel: model}
	device.ControlPTZ(v.Command, v.ChannelID)

	return "OK", nil
}
