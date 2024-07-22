package main

import (
	"github.com/lkmio/avformat/transport"
	"go.uber.org/zap/zapcore"
)

var (
	Config           *Config_
	SipUA            SipServer
	TransportManager transport.Manager
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
	TransportManager = transport.NewTransportManager(uint16(Config.Port[0]), uint16(Config.Port[1]))

	db := &MemoryDB{}
	devices := db.LoadDevices()
	for _, device := range devices {
		DeviceManager.Add(device)
	}

	server, err := StartSipServer(config, db)
	if err != nil {
		panic(err)
	}
	SipUA = server

	startApiServer(config.HttpAddr)
}
