package main

import (
	"github.com/lkmio/avformat/utils"
	"time"
)

// 启动级联设备
func startPlatformDevices() {
	platforms, err := PlatformDao.LoadPlatforms()
	if err != nil {
		Sugar.Errorf("查询级联设备失败 err: %s", err.Error())
		return
	}

	for _, record := range platforms {
		platform, err := NewGBPlatform(record, SipUA)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(PlatformManager.Add(platform))

		if err := PlatformDao.UpdatePlatformStatus(record.ServerAddr, OFF); err != nil {
			Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.SeverID)
		}

		// 恢复级联会话
		// 不删会话能正常通信
		//for _, stream := range streams {
		//	sinks := stream.GetForwardStreamSinks()
		//	for _, sink := range sinks {
		//		if sink.DeviceID != record.SeverID {
		//			continue
		//		}
		//
		//		callId, _ := sink.Dialog.CallID()
		//		channelCallId, _ := stream.Dialog.CallID()
		//		platform.addSink(callId.Value(), channelCallId.Value())
		//	}
		//}

		platform.Start()
	}
}

// 返回需要关闭的推流源和转流Sink
func recoverStreams() (map[string]*Stream, map[string]*Sink) {
	// 比较数据库和流媒体服务器中的流会话, 以流媒体服务器中的为准, 释放过期的会话
	// source id和stream id目前都是同一个id
	dbStreams, err := StreamDao.LoadStreams()
	if err != nil {
		Sugar.Errorf("恢复推流失败, 查询数据库发生错误. err: %s", err.Error())
		return nil, nil
	}

	dbSinks, _ := SinkDao.LoadForwardSinks()

	// 查询流媒体服务器中的推流源列表
	msSources, err := QuerySourceList()
	if err != nil {
		// 流媒体服务器崩了, 存在的所有记录都无效, 全部删除
		Sugar.Warnf("恢复推流失败, 查询推流源列表发生错误, 删除数据库中的所有记录. err: %s", err.Error())
	}

	// 查询推流源下所有的转发sink列表
	msStreamSinks := make(map[string]string, len(msSources))
	for _, source := range msSources {
		// 跳过非国标流
		if "28181" != source.Protocol && "gb_talk" != source.Protocol {
			continue
		}

		// 查询转发sink
		sinks, err := QuerySinkList(source.ID)
		if err != nil {
			Sugar.Warnf("查询拉流列表发生 err: %s", err.Error())
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

	_ = StreamDao.DeleteStreamsByIds(invalidStreamIds)
	_ = SinkDao.DeleteForwardSinksByIds(invalidSinkIds)
	return dbStreams, dbSinks
}

// 更新设备的在线状态
func updateDevicesStatus() {
	devices, err := DeviceDao.LoadDevices()
	if err != nil {
		panic(err)
	} else if len(devices) > 0 {
		now := time.Now()
		var offlineDevices []string
		for key, device := range devices {
			if device.Status == OFF {
				continue
			} else if now.Sub(device.LastHeartbeat) < time.Duration(Config.AliveExpires)*time.Second {
				OnlineDeviceManager.Add(key, device.LastHeartbeat)
				continue
			}

			offlineDevices = append(offlineDevices, key)
		}

		if len(offlineDevices) > 0 {
			if err = DeviceDao.UpdateOfflineDevices(offlineDevices); err != nil {
				Sugar.Errorf("更新设备状态失败 device: %s", offlineDevices)
			}
		}
	}
}
