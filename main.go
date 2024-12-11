package main

import (
	"encoding/json"
	"github.com/lkmio/avformat/transport"
	"github.com/lkmio/avformat/utils"
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

	// 设置语音广播端口
	TransportManager = transport.NewTransportManager(uint16(Config.Port[0]), uint16(Config.Port[1]))

	// 启动sip server
	server, err := StartSipServer(config.SipId, config.ListenIP, config.PublicIP, config.SipPort)
	if err != nil {
		panic(err)
	}

	Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	SipUA = server

	// 启动级联设备
	platforms, err := DB.LoadPlatforms()
	for _, record := range platforms {
		platform, err := NewGBPlatform(record, SipUA)
		// 都入库了不允许失败, 程序有BUG, 及时修复
		utils.Assert(err == nil)
		utils.Assert(PlatformManager.AddPlatform(platform))

		if err := DB.UpdatePlatformStatus(record.SeverID, OFF); err != nil {
			Sugar.Infof("更新级联设备状态失败 err: %s device: %s", err.Error(), record.SeverID)
		}

		platform.Start()
	}

	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	Sugar.Infof("启动http server. addr: %s", httpAddr)
	startApiServer(httpAddr)
}
