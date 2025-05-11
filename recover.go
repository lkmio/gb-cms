package main

import "github.com/lkmio/avformat/utils"

// 启动级联设备
func startPlatformDevices() {
	platforms, err := DB.LoadPlatforms()
	if err != nil {
		Sugar.Errorf("查询级联设备失败 err: %s", err.Error())
		return
	}

	//streams := StreamManager.All()
	for _, record := range platforms {
		platform, err := NewGBPlatform(record, SipUA)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(PlatformManager.Add(platform))

		if err := DB.UpdatePlatformStatus(record.ServerAddr, OFF); err != nil {
			Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.SeverID)
		}

		// 恢复级联会话
		// 不删会话能正常通信
		//for _, stream := range streams {
		//	sinks := stream.GetForwardStreamSinks()
		//	for _, sink := range sinks {
		//		if sink.ID != record.SeverID {
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

func closeStream(stream *Stream) {
	DB.DeleteStream(stream.CreateTime)
	// 删除转发sink
	DB.DeleteForwardSinks(stream.ID)
}

// 返回需要关闭的推流源和转流Sink
func recoverStreams() ([]*Stream, []*Sink) {
	// 比较数据库和流媒体服务器中的流会话, 以流媒体服务器中的为准, 释放过期的会话
	// source id和stream id目前都是同一个id
	dbStreams, err := DB.LoadStreams()
	if err != nil {
		Sugar.Errorf("恢复推流失败, 查询数据库发生错误. err: %s", err.Error())
		return nil, nil
	} else if len(dbStreams) < 1 {
		return nil, nil
	}

	var closedStreams []*Stream
	var closedSinks []*Sink

	// 查询流媒体服务器中的推流源列表
	sources, err := QuerySourceList()
	if err != nil {
		// 流媒体服务器崩了, 存在的所有记录都无效, 全部删除
		Sugar.Warnf("恢复推流失败, 查询推流源列表发生错误, 删除数据库中的所有记录. err: %s", err.Error())

		for _, stream := range dbStreams {
			closedStreams = append(closedStreams, stream)
		}
		return closedStreams, nil
	}

	// 查询推流源下所有的转发sink列表
	msStreamSinks := make(map[string]map[string]string, len(sources))
	for _, source := range sources {
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

		stream, ok := dbStreams[source.ID]
		if !ok {
			Sugar.Warnf("流媒体中的流不存在于数据库中 source: %s", source.ID)
			continue
		}

		stream.SinkCount = int32(len(sinks))
		forwardSinks := make(map[string]string, len(sinks))
		for _, sink := range sinks {
			if "gb_cascaded_forward" == sink.Protocol || "gb_talk_forward" == sink.Protocol {
				forwardSinks[sink.ID] = ""
			}
		}

		msStreamSinks[source.ID] = forwardSinks
	}

	// 遍历数据库中的流会话, 比较是否存在于流媒体服务器中, 不存在则删除
	for _, stream := range dbStreams {
		// 如果stream不存在于流媒体服务器中, 则删除
		msSinks, ok := msStreamSinks[string(stream.ID)]
		if !ok {
			Sugar.Infof("删除过期的推流会话 stream: %s", stream.ID)
			closedStreams = append(closedStreams, stream)
			continue
		}

		// 查询stream下的转发sink列表
		dbSinks, err := DB.QueryForwardSinks(stream.ID)
		if err != nil {
			Sugar.Errorf("查询级联转发sink列表失败 err: %s", err.Error())
		}

		// 遍历数据库中的sink, 如果不存在于流媒体服务器中, 则删除
		for _, sink := range dbSinks {
			_, ok := msSinks[sink.ID]
			if ok {
				// 恢复转发sink
				AddForwardSink(sink.Stream, sink)
				if sink.Protocol == "gb_talk_forward" {
					SinkManager.AddWithSinkStreamId(sink)
				}
			} else {
				Sugar.Infof("删除过期的级联转发会话 stream: %s sink: %s", stream.ID, sink.ID)
				closedSinks = append(closedSinks, sink)
			}
		}

		Sugar.Infof("恢复推流会话 stream: %s", stream.ID)

		StreamManager.Add(stream)
		if stream.Dialog != nil {
			callId, _ := stream.Dialog.CallID()
			StreamManager.AddWithCallId(callId.Value(), stream)
		}
	}

	return closedStreams, closedSinks
}

// 更新设备的在线状态
func updateDevicesStatus() {
	onlineDevices, err := DB.LoadOnlineDevices()
	if err != nil {
		panic(err)
	}

	devices, err := DB.LoadDevices()
	if err != nil {
		panic(err)
	} else if len(devices) > 0 {

		for key, device := range devices {
			status := OFF
			if _, ok := onlineDevices[key]; ok {
				status = ON
			}

			// 根据通道在线状态，统计通道总数和离线数量
			var total int
			var online int
			channels, _, err := DB.QueryChannels(key, 1, 0xFFFFFFFF)
			if err != nil {
				Sugar.Errorf("查询通道列表失败 err: %s device: %s", err.Error(), key)
			} else {
				total = len(channels)
				for _, channel := range channels {
					if channel.Online() {
						online++
					}
				}
			}

			device.ChannelsTotal = total
			device.ChannelsOnline = online
			device.Status = status
			if err = DB.SaveDevice(device); err != nil {
				Sugar.Errorf("更新设备状态失败 device: %s status: %s", key, status)
				continue
			}

			DeviceManager.Add(device)
		}
	}
}
