package main

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/lkmio/avformat/utils"
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
		platform, err := stack.NewPlatform(&record.SIPUAOptions, common.SipStack)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(stack.PlatformManager.Add(platform.ServerAddr, platform))

		if err := dao.Platform.UpdateOnlineStatus(common.OFF, record.ServerAddr); err != nil {
			log.Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.ServerID)
		}

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
		device, err := stack.NewJTDevice(record, common.SipStack)
		utils.Assert(err == nil)
		utils.Assert(stack.JTDeviceManager.Add(device.Username, device))

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

	dbSinks, _ := dao.Sink.LoadForwardSinks()

	// 查询流媒体服务器中的推流源列表
	msSources, err := stack.MSQuerySourceList()
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
		sinks, err := stack.MSQuerySinkList(source.ID)
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
	_ = dao.Sink.DeleteForwardSinksByIds(invalidSinkIds)
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
				stack.OnlineDeviceManager.Add(key, device.LastHeartbeat)
				continue
			}

			offlineDevices = append(offlineDevices, key)
		}

		if len(offlineDevices) > 0 {
			// 每次更新100个
			for i := 0; i < len(offlineDevices); i += 100 {
				end := i + 100
				if end > len(offlineDevices) {
					end = len(offlineDevices)
				}
				if err = dao.Device.UpdateOfflineDevices(offlineDevices[i:end]); err != nil {
					log.Sugar.Errorf("更新设备状态失败 device: %s", offlineDevices[i:end])
				}
			}
		}
	}
}
