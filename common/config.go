package common

import (
	"encoding/json"
	"os"
	"time"
)

var (
	Config *Config_
)

type Config_ struct {
	SipPort  int    `json:"sip_port"`
	HttpPort int    `json:"http_port"`
	ListenIP string `json:"listen_ip"`
	PublicIP string `json:"public_ip"`

	SipID          string `json:"sip_id"`
	Password       string `json:"password"`
	SipContactAddr string

	AliveExpires           int `json:"alive_expires"`
	MobilePositionInterval int `json:"mobile_position_interval"`
	SubscribeExpires       int `json:"subscribe_expires"`
	PositionReserveDays    int `json:"position_reserve_days"`
	AlarmReserveDays       int `json:"alarm_reserve_days"`

	MediaServer string `json:"media_server"`

	Hooks struct {
		Online   string `json:"online"`
		Offline  string `json:"offline"`
		Position string `json:"position"`
		OnInvite string `json:"on_invite"`
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

func ParseGBTime(gbTime string) time.Time {
	// 2023-08-10 15:04:05
	if gbTime == "" {
		return time.Time{}
	}

	// 解析时间字符串
	t, err := time.Parse("2006-01-02T15:04:05", gbTime)
	if err != nil {
		return time.Time{}
	}

	return t
}
