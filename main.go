package main

import (
	"encoding/json"
	"github.com/lkmio/avformat/transport"
	"go.uber.org/zap/zapcore"
	"net"
	"strconv"
)

var (
	Config           *Config_
	SipUA            SipServer
	TransportManager transport.Manager
	DB               DeviceDB
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

	TransportManager = transport.NewTransportManager(uint16(Config.Port[0]), uint16(Config.Port[1]))

	DB = &LocalDB{}
	devices := DB.LoadDevices()
	for _, device := range devices {
		DeviceManager.Add(device)
	}

	server, err := StartSipServer(config)
	if err != nil {
		panic(err)
	}

	Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	SipUA = server

	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	Sugar.Infof("启动http server. addr: %s", httpAddr)
	startApiServer(httpAddr)
}
