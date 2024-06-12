package main

import (
	"go.uber.org/zap/zapcore"
)

var (
	Config *Config_
	SipUA  SipServer
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
