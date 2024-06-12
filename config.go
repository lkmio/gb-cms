package main

import (
	"encoding/json"
	"github.com/ghettovoice/gosip/sip"
	"os"
)

type Config_ struct {
	SipAddr                string `json:"sip_addr"`
	HttpAddr               string `json:"http_addr"`
	PublicIP               string `json:"public_ip"`
	SipId                  string `json:"sip_id"`
	SipRealm               string `json:"sip_realm"`
	Password               string `json:"password"`
	AliveExpires           int    `json:"alive_expires"`
	MobilePositionInterval int    `json:"mobile_position_interval"`
	MediaServer            string `json:"media_server"`

	SipPort sip.Port
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
