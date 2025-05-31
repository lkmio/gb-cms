package main

import (
	"encoding/json"
	"gb-cms/hook"
	"go.uber.org/zap/zapcore"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"
)

var (
	Config   *Config_
	SipStack SipServer
)

func init() {
	logConfig := LogConfig{
		Level:     int(zapcore.DebugLevel),
		Name:      "./logs/clog",
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

	if config.Hooks.OnInvite != "" {
		hook.RegisterEventUrl(hook.EventTypeDeviceOnInvite, config.Hooks.OnInvite)
	}

	OnlineDeviceManager.Start(time.Duration(Config.AliveExpires)*time.Second/4, time.Duration(Config.AliveExpires)*time.Second, OnExpires)

	// 从数据库中恢复会话
	var streams map[string]*Stream
	var sinks map[string]*Sink

	// 查询在线设备, 更新设备在线状态
	updateDevicesStatus()

	// 恢复国标推流会话
	streams, sinks = recoverStreams()

	// 启动sip server
	server, err := StartSipServer(config.SipID, config.ListenIP, config.PublicIP, config.SipPort)
	if err != nil {
		panic(err)
	}

	Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	SipStack = server

	// 在sip启动后, 关闭无效的流
	for _, stream := range streams {
		stream.Bye()
	}

	for _, sink := range sinks {
		sink.Bye()
	}

	// 启动级联设备
	startPlatformDevices()
	// 启动1078设备
	startJTDevices()

	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	Sugar.Infof("启动http server. addr: %s", httpAddr)
	go startApiServer(httpAddr)

	err = http.ListenAndServe(":19000", nil)
	if err != nil {
		println(err)
	}
}
