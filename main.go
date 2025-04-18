package main

import (
	"encoding/json"
	"github.com/lkmio/avformat/utils"
	"github.com/lkmio/transport"
	"go.uber.org/zap/zapcore"
	"net"
	"strconv"
)

var (
	Config           *Config_
	SipUA            SipServer
	TransportManager transport.Manager
	DB               GB28181DB
)

func init() {
	logConfig := LogConfig{
		Level:     int(zapcore.DebugLevel),
		Name:      "./logs/cms.log",
		MaxSize:   10,
		MaxBackup: 100,
		MaxAge:    7,
		Compress:  false,
	}

	InitLogger(zapcore.Level(logConfig.Level), logConfig.Name, logConfig.MaxSize, logConfig.MaxBackup, logConfig.MaxAge, logConfig.Compress)
}

func startPlatformDevices() {
	platforms, err := DB.LoadPlatforms()
	if err != nil {
		Sugar.Errorf("查询级联设备失败 err: %s", err.Error())
		return
	}

	streams := StreamManager.All()
	for _, record := range platforms {
		platform, err := NewGBPlatform(record, SipUA)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(PlatformManager.AddPlatform(platform))

		if err := DB.UpdatePlatformStatus(record.SeverID, OFF); err != nil {
			Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.SeverID)
		}

		// 恢复级联会话
		for _, stream := range streams {
			sinks := stream.GetForwardStreamSinks()
			for _, sink := range sinks {
				if sink.ID != record.SeverID {
					continue
				}

				callId, _ := sink.Dialog.CallID()
				channelCallId, _ := stream.Dialog.CallID()
				platform.AddStream(callId.Value(), channelCallId.Value())
			}
		}

		platform.Start()
	}
}

func recoverStreams() ([]*Stream, []*Sink) {
	// 查询数据库中的流记录
	// 查询流媒体服务器中的记录
	// 合并两份记录, 以流媒体服务器中的为准。如果流记录数量不一致(只会时数据库中的记录数大于或等于流媒体中的记录数), 释放过期的会话.
	// source id和stream id目前都是同一个id

	streams, err := DB.LoadStreams()
	if err != nil {
		Sugar.Errorf("恢复推流失败, 查询数据库发生错误. err: %s", err.Error())
		return nil, nil
	} else if len(streams) < 1 {
		return nil, nil
	}

	sources, err := QuerySourceList()
	if err != nil {
		// 流媒体服务器崩了, 存在的所有流都无效, 删除全部记录
		Sugar.Warnf("恢复推流失败, 查询推流源列表发生错误, 删除数据库中的推拉流会话记录. err: %s", err.Error())

		for _, stream := range streams {
			DB.DeleteStream(stream.CreateTime)
		}

		return nil, nil
	}

	sourceSinks := make(map[string][]string, len(sources))
	for _, source := range sources {
		// 跳过非国标流
		if "28181" != source.Protocol {
			continue
		}

		// 查询级联转发sink
		sinks, err := QuerySinkList(source.ID)
		if err != nil {
			Sugar.Warnf("查询拉流列表发生 err: %s", err.Error())
			continue
		}

		stream, ok := streams[source.ID]
		utils.Assert(ok)
		stream.SinkCount = int32(len(sinks))

		var forwardSinks []string
		for _, sink := range sinks {
			if "gb_stream_forward" == sink.Protocol {
				forwardSinks = append(forwardSinks, sink.ID)
			}
		}

		sourceSinks[source.ID] = forwardSinks
	}

	var closedStreams []*Stream
	var closedSinks []*Sink
	for _, stream := range streams {
		forwardSinks, ok := sourceSinks[string(stream.ID)]
		if !ok {
			Sugar.Infof("删除过期的推流会话 stream: %s", stream.ID)
			closedStreams = append(closedStreams, stream)
			continue
		}

		Sugar.Infof("恢复推流会话 stream: %s", stream.ID)

		var invalidDialogs []string
		for callId, sink := range stream.ForwardStreamSinks {
			var exist bool
			for _, id := range forwardSinks {
				if id == sink.ID {
					exist = true
					break
				}
			}

			if !exist {
				Sugar.Infof("删除过期的级联转发会话 stream: %s sink: %s callId: %s", stream.ID, sink.ID, callId)
			}

			invalidDialogs = append(invalidDialogs, callId)
		}

		for _, id := range invalidDialogs {
			sink := stream.RemoveForwardStreamSink(id)
			closedSinks = append(closedSinks, sink)
		}

		StreamManager.Add(stream)
		callId, _ := stream.Dialog.CallID()
		StreamManager.AddWithCallId(callId.Value(), stream)
	}

	return closedStreams, closedSinks
}

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

func main() {
	config, err := ParseConfig("./config.json")
	if err != nil {
		panic(err)
	}

	Config = config
	indent, _ := json.MarshalIndent(Config, "", "\t")
	Sugar.Infof("server config:\r\n%s", indent)

	DB = NewRedisDB(Config.Redis.Addr, Config.Redis.Password)

	// 查询在线设备, 更新设备在线状态
	updateDevicesStatus()

	// 恢复国标推流会话
	streams, sinks := recoverStreams()

	// 设置语音广播端口
	TransportManager = transport.NewTransportManager(Config.ListenIP, uint16(Config.Port[0]), uint16(Config.Port[1]))

	// 启动sip server
	server, err := StartSipServer(config.SipId, config.ListenIP, config.PublicIP, config.SipPort)
	if err != nil {
		panic(err)
	}

	Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	SipUA = server

	// 在sip启动后, 关闭无效的流
	for _, stream := range streams {
		stream.Close(true, false)
	}

	for _, sink := range sinks {
		sink.Close(true, false)
	}

	// 启动级联设备
	startPlatformDevices()

	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	Sugar.Infof("启动http server. addr: %s", httpAddr)
	startApiServer(httpAddr)
}
