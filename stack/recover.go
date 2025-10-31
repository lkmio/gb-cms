package stack

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/go-co-op/gocron/v2"
	"github.com/lkmio/avformat/utils"
	"net"
	"strconv"
	"time"
)

// 启动级联设备
func startPlatformDevices() {
	platforms, err := dao.Platform.LoadPlatforms()
	if err != nil {
		log.Sugar.Errorf("查询级联设备失败 err: %s", err.Error())
		return
	}

	for _, record := range platforms {
		if err := dao.Platform.UpdateOnlineStatus(common.OFF, record.ServerAddr); err != nil {
			log.Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.ServerID)
		}

		if !record.Enable {
			continue
		}

		platform, err := NewPlatform(&record.SIPUAOptions, common.SipStack)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(PlatformManager.Add(platform.ServerAddr, platform))

		platform.Start()
	}
}

// 启动1078设备
func startJTDevices() {
	devices, err := dao.JTDevice.LoadDevices()
	if err != nil {
		log.Sugar.Errorf("查询1078设备失败 err: %s", err.Error())
		return
	}

	for _, record := range devices {
		// 都入库了不允许失败, 程序有BUG, 及时修复
		device, err := NewJTDevice(record, common.SipStack)
		utils.Assert(err == nil)
		utils.Assert(JTDeviceManager.Add(device.Username, device))

		if err := dao.JTDevice.UpdateOnlineStatus(common.OFF, device.Username); err != nil {
			log.Sugar.Infof("更新1078设备状态失败 err: %s device: %s", err.Error(), record.SeverID)
		}
		device.Start()
	}
}

// 返回需要关闭的推流源和转流Sink
func recoverStreams() (map[string]*dao.StreamModel, map[string]*dao.SinkModel) {
	// 比较数据库和流媒体服务器中的流会话, 以流媒体服务器中的为准, 释放过期的会话
	// source id和stream id目前都是同一个id
	dbStreams, err := dao.Stream.LoadStreams()
	if err != nil {
		log.Sugar.Errorf("恢复推流失败, 查询数据库发生错误. err: %s", err.Error())
		return nil, nil
	}

	dbSinks, _ := dao.Sink.LoadSinks()

	// 查询流媒体服务器中的推流源列表
	msSources, err := MSQuerySourceList()
	if err != nil {
		// 流媒体服务器崩了, 存在的所有记录都无效, 全部删除
		log.Sugar.Warnf("恢复推流失败, 查询推流源列表发生错误, 删除所有推流记录. err: %s", err.Error())
	}

	// 查询推流源下所有的转发sink列表
	msStreamSinks := make(map[string]string, len(msSources))
	for _, source := range msSources {
		// 跳过非国标流
		if "28181" != source.Protocol && "gb_talk" != source.Protocol {
			continue
		}

		// 查询转发sink
		sinks, err := MSQuerySinkList(source.ID)
		if err != nil {
			log.Sugar.Warnf("查询拉流列表发生 err: %s", err.Error())
			continue
		}

		for _, sink := range sinks {
			if "gb_cascaded_forward" == sink.Protocol || "gb_talk_forward" == sink.Protocol {
				msStreamSinks[sink.ID] = source.ID
			}
		}
	}

	for _, source := range msSources {
		delete(dbStreams, source.ID)
	}

	for key, _ := range msStreamSinks {
		if dbSinks != nil {
			delete(dbSinks, key)
		}
	}

	var invalidStreamIds []uint
	for _, stream := range dbStreams {
		invalidStreamIds = append(invalidStreamIds, stream.ID)
	}

	var invalidSinkIds []uint
	for _, sink := range dbSinks {
		invalidSinkIds = append(invalidSinkIds, sink.ID)
	}

	_ = dao.Stream.DeleteStreamsByIds(invalidStreamIds)
	_ = dao.Sink.DeleteSinksByIds(invalidSinkIds)
	return dbStreams, dbSinks
}

// 更新设备的在线状态
func updateDevicesStatus() {
	devices, err := dao.Device.LoadDevices()
	if err != nil {
		panic(err)
	} else if len(devices) > 0 {
		now := time.Now()
		var offlineDevices []string
		for key, device := range devices {
			if device.Status == common.OFF {
				continue
			} else if now.Sub(device.LastHeartbeat) < time.Duration(common.Config.AliveExpires)*time.Second {
				OnlineDeviceManager.Add(key, device.LastHeartbeat)
				continue
			}

			offlineDevices = append(offlineDevices, key)
		}

		for _, device := range offlineDevices {
			CloseDevice(device)
		}
	}
}

func closeStreamsAndSinks(streams map[string]*dao.StreamModel, sinks map[string]*dao.SinkModel, ms bool) {
	for _, stream := range streams {
		(&Stream{StreamModel: stream}).Close(true, ms)
	}

	for _, sink := range sinks {
		(&Sink{SinkModel: sink}).Close(true, ms)
	}
}

func closeAllStreamsAndSinks() {
	dbStreams, _ := dao.Stream.LoadStreams()
	dbSinks, _ := dao.Sink.LoadSinks()
	closeStreamsAndSinks(dbStreams, dbSinks, true)
}

func RestartSipStack(newConfig *common.Config_) error {
	// 关闭所有推拉流/对讲/级联等会话
	closeAllStreamsAndSinks()

	// 停止所有级联设备
	platforms := PlatformManager.All()
	for _, platform := range platforms {
		platform.Stop()
	}

	sipLock.Lock()
	defer func() {
		sipLock.Unlock()

		// 重启级联设备
		go func() {
			for _, platform := range PlatformManager.All() {
				platform.Start()
			}
		}()
	}()

	// 重启sip协议栈
	common.Config = newConfig
	err := common.SipStack.Restart(newConfig.SipID, newConfig.ListenIP, newConfig.PublicIP, newConfig.SipPort)
	if err != nil {
		log.Sugar.Errorf("重启sip服务器失败. err: %s", err.Error())
		return err
	}

	common.Config.SipContactAddr = net.JoinHostPort(newConfig.PublicIP, strconv.Itoa(newConfig.SipPort))
	return nil
}

func Start() {
	// 启动设备在线超时管理
	OnlineDeviceManager.Start(time.Duration(common.Config.AliveExpires)*time.Second/4, time.Duration(common.Config.AliveExpires)*time.Second, OnExpires)

	// 查询在线设备, 更新设备在线状态
	updateDevicesStatus()

	// 恢复国标推流会话
	streams, sinks := recoverStreams()

	// 在sip启动后, 关闭无效的流
	closeStreamsAndSinks(streams, sinks, false)

	// 启动级联设备
	startPlatformDevices()

	// 启动1078设备
	startJTDevices()

	// 启动目录刷新任务
	go AddScheduledTask(time.Minute, true, RefreshCatalogScheduleTask)
	// 启动订阅刷新任务
	go AddScheduledTask(time.Minute, true, RefreshSubscribeScheduleTask)

	// 启动定时任务, 每天凌晨3点执行
	s, _ := gocron.NewScheduler()
	defer func() { _ = s.Shutdown() }()

	// 删除过期的位置、报警记录
	_, _ = s.NewJob(
		gocron.CronJob(
			"0 3 * * *",
			false,
		),
		gocron.NewTask(
			func() {
				now := time.Now()
				alarmExpireTime := now.AddDate(0, 0, -common.Config.AlarmReserveDays)
				positionExpireTime := now.AddDate(0, 0, -common.Config.PositionReserveDays)

				// 删除过期的报警记录
				err := dao.Alarm.DeleteExpired(alarmExpireTime)
				if err != nil {
					log.Sugar.Errorf("删除过期的报警记录失败 err: %s", err.Error())
				}
				// 删除过期的位置记录
				err = dao.Position.DeleteExpired(positionExpireTime)
				if err != nil {
					log.Sugar.Errorf("删除过期的位置记录失败 err: %s", err.Error())
				}
			},
		),
	)

	s.Start()

	// 加载ip2region数据库
	if common.Config.IP2RegionEnable {
		err := common.LoadIP2RegionDB(common.Config.IP2RegionDBPath)
		if err != nil {
			common.Config.IP2RegionEnable = false
			log.Sugar.Errorf("加载ip2region数据库失败. err: %s", err.Error())
		}
	}
}
