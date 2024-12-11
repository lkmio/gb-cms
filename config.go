package main

import (
	"encoding/json"
	"os"
)

type Config_ struct {
	SipPort  int    `json:"sip_port"`
	HttpPort int    `json:"http_port"`
	ListenIP string `json:"listen_ip"`
	PublicIP string `json:"public_ip"`

	SipId          string `json:"sip_id"`
	Password       string `json:"password"`
	SipContactAddr string

	AliveExpires           int    `json:"alive_expires"`
	MobilePositionInterval int    `json:"mobile_position_interval"`
	MobilePositionExpires  int    `json:"mobile_position_expires"`
	MediaServer            string `json:"media_server"`
	Port                   []int  `json:"port"` //语音广播/对讲需要的端口

	Redis struct {
		Addr     string `json:"addr"`
		Password string `json:"password"`
	}
}

type LogConfig struct {
	Level     int
	Name      string
	MaxSize   int
	MaxBackup int
	MaxAge    int
	Compress  bool
}

func ParseConfig(path string) (*Config_, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := Config_{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		return nil, err
	}

	return &config, err
}
