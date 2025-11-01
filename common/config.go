package common

import (
	"gopkg.in/ini.v1"
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
	Realm          string
	Password       string `json:"password"`
	SipContactAddr string

	AliveExpires           int `json:"alive_expires"`
	MobilePositionInterval int `json:"mobile_position_interval"`
	SubscribeExpires       int `json:"subscribe_expires"`
	PositionReserveDays    int `json:"position_reserve_days"`
	AlarmReserveDays       int `json:"alarm_reserve_days"`
	LogReserveDays         int `json:"log_reserve_days"`

	MediaServer     string `json:"media_server"`
	PreferStreamFmt string `json:"prefer_stream_fmt"`
	InviteTimeout   int

	SubCatalogGlobalInterval  int `json:"sub_catalog_global_interval"`
	SubAlarmGlobalInterval    int `json:"sub_alarm_global_interval"`
	SubPositionGlobalInterval int `json:"sub_position_global_interval"`
	SubPTZGlobalInterval      int `json:"sub_ptz_global_interval"`

	GlobalDropChannelType string `json:"global_drop_channel_type"`

	DeviceDefaultMediaTransport string `json:"device_default_media_transport"`

	Hooks struct {
		Online   string `json:"online"`
		Offline  string `json:"offline"`
		Position string `json:"position"`
		OnInvite string `json:"on_invite"`
	}

	IP2RegionDBPath string
	IP2RegionEnable bool
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
	load, err := ini.Load(path)
	if err != nil {
		return nil, err
	}

	config_ := Config_{
		SipPort:                     load.Section("sip").Key("port").MustInt(),
		HttpPort:                    load.Section("http").Key("port").MustInt(),
		ListenIP:                    load.Section("sip").Key("listen_ip").String(),
		PublicIP:                    load.Section("sip").Key("public_ip").String(),
		SipID:                       load.Section("sip").Key("id").String(),
		Realm:                       load.Section("sip").Key("realm").String(),
		Password:                    load.Section("sip").Key("password").String(),
		AliveExpires:                load.Section("sip").Key("alive_expires").MustInt(),
		MobilePositionInterval:      load.Section("sip").Key("mobile_position_interval").MustInt(),
		SubscribeExpires:            load.Section("sip").Key("subscribe_expires").MustInt(),
		PositionReserveDays:         load.Section("sip").Key("position_reserve_days").MustInt(),
		AlarmReserveDays:            load.Section("sip").Key("alarm_reserve_days").MustInt(),
		LogReserveDays:              load.Section("sip").Key("log_reserve_days").MustInt(),
		MediaServer:                 load.Section("sip").Key("media_server").String(),
		PreferStreamFmt:             load.Section("sip").Key("prefer_stream_fmt").String(),
		InviteTimeout:               load.Section("sip").Key("invite_timeout").MustInt(),
		SubCatalogGlobalInterval:    load.Section("sip").Key("sub_catalog_global_interval").MustInt(),
		SubAlarmGlobalInterval:      load.Section("sip").Key("sub_alarm_global_interval").MustInt(),
		SubPositionGlobalInterval:   load.Section("sip").Key("sub_position_global_interval").MustInt(),
		SubPTZGlobalInterval:        load.Section("sip").Key("sub_ptz_global_interval").MustInt(),
		DeviceDefaultMediaTransport: load.Section("sip").Key("device_default_media_transport").String(),
		GlobalDropChannelType:       load.Section("sip").Key("global_drop_channel_type").String(),
		IP2RegionDBPath:             load.Section("ip2region").Key("db_path").String(),
		IP2RegionEnable:             load.Section("ip2region").Key("enable").MustBool(),
	}

	config_.Hooks.Online = load.Section("hooks").Key("online").String()
	config_.Hooks.Offline = load.Section("hooks").Key("offline").String()
	config_.Hooks.Position = load.Section("hooks").Key("position").String()
	config_.Hooks.OnInvite = load.Section("hooks").Key("on_invite").String()

	return &config_, err
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
