package api

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"gopkg.in/ini.v1"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var (
	setConfigLock sync.Mutex
)

type BaseConfig struct {
	APIAuth                               bool   `json:"APIAuth"`
	AckTimeout                            int    `json:"AckTimeout"`
	AllowStreamStartByURL                 bool   `json:"AllowStreamStartByURL"`
	BlackIPList                           string `json:"BlackIPList"`
	BlackUAList                           string `json:"BlackUAList"`
	Captcha                               bool   `json:"Captcha"`
	DevicePassword                        string `json:"DevicePassword"`
	DropChannelType                       string `json:"DropChannelType"`
	GlobalChannelAudio                    bool   `json:"GlobalChannelAudio"`
	GlobalChannelShared                   bool   `json:"GlobalChannelShared"`
	GlobalDeviceAlarmSubscribeInterval    int    `json:"GlobalDeviceAlarmSubscribeInterval"`
	GlobalDeviceCatalogSubscribeInterval  int    `json:"GlobalDeviceCatalogSubscribeInterval"`
	GlobalDevicePTZSubscribeInterval      int    `json:"GlobalDevicePTZSubscribeInterval"`
	GlobalDevicePositionSubscribeInterval int    `json:"GlobalDevicePositionSubscribeInterval"`
	HTTPSCertFile                         string `json:"HTTPSCertFile"`
	HTTPSKeyFile                          string `json:"HTTPSKeyFile"`
	HTTPSPort                             int    `json:"HTTPSPort"`
	Host                                  string `json:"Host"`
	KeepaliveTimeout                      int    `json:"KeepaliveTimeout"`
	LiveStreamAuth                        bool   `json:"LiveStreamAuth"`
	MapCenter                             string `json:"MapCenter"`
	MapEnable                             bool   `json:"MapEnable"`
	MediaTransport                        string `json:"MediaTransport"`
	MediaTransportMode                    string `json:"MediaTransportMode"`
	Port                                  int    `json:"Port"`
	PreferStreamFmt                       string `json:"PreferStreamFmt"`
	Realm                                 string `json:"Realm"`
	SIPLog                                bool   `json:"SIPLog"`
	Serial                                string `json:"Serial"`
	TimeServer                            string `json:"TimeServer"`
}

func (api *ApiServer) OnGetBaseConfig(_ *Empty, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	setupType := common.String2SetupType(common.Config.DeviceDefaultMediaTransport)
	ip, ua := dao.BlacklistManager.ToStrings()
	v := BaseConfig{
		APIAuth:                               true,
		AckTimeout:                            common.Config.InviteTimeout,
		AllowStreamStartByURL:                 false,
		BlackIPList:                           ip,
		BlackUAList:                           ua,
		Captcha:                               false,
		DevicePassword:                        common.Config.Password,
		GlobalChannelAudio:                    true,
		GlobalChannelShared:                   false,
		GlobalDeviceAlarmSubscribeInterval:    common.Config.SubAlarmGlobalInterval,
		GlobalDeviceCatalogSubscribeInterval:  common.Config.SubCatalogGlobalInterval,
		GlobalDevicePTZSubscribeInterval:      common.Config.SubPTZGlobalInterval,
		GlobalDevicePositionSubscribeInterval: common.Config.SubPositionGlobalInterval,
		HTTPSCertFile:                         "",
		HTTPSKeyFile:                          "",
		HTTPSPort:                             0,
		Host:                                  common.Config.PublicIP,
		KeepaliveTimeout:                      common.Config.AliveExpires,
		LiveStreamAuth:                        true,
		MapCenter:                             "",
		MapEnable:                             false,
		MediaTransport:                        setupType.Transport(),
		MediaTransportMode:                    setupType.String(),
		Port:                                  common.Config.SipPort,
		PreferStreamFmt:                       common.Config.PreferStreamFmt,
		Realm:                                 common.Config.Realm,
		SIPLog:                                false,
		Serial:                                common.Config.SipID,
		TimeServer:                            "",
	}

	return v, nil
}

func (api *ApiServer) OnSetBaseConfig(baseConfig *BaseConfig, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	if len(baseConfig.Serial) != 20 {
		return nil, fmt.Errorf("serial must be 20 characters long")
	} else if len(baseConfig.Realm) == 0 {
		return nil, fmt.Errorf("realm must be 10 characters long")
	} else if baseConfig.Port == 0 {
		return nil, fmt.Errorf("port must be greater than 0")
	}

	setConfigLock.Lock()
	defer setConfigLock.Unlock()

	// 黑名单
	ipList := strings.Split(baseConfig.BlackIPList, ",")
	uaList := strings.Split(baseConfig.BlackUAList, ",")
	err := dao.Blacklist.Replace(ipList, uaList)
	if err != nil {
		log.Sugar.Errorf("更新黑名单失败: %s", err.Error())
		return nil, err
	}

	iniConfig, err := ini.Load("./config.ini")
	if err != nil {
		return nil, err
	}

	var sipChanged bool
	var changed bool
	if baseConfig.Serial != common.Config.SipID {
		// 更新sip id
		iniConfig.Section("sip").Key("id").SetValue(baseConfig.Serial)
		sipChanged = true
	}

	if baseConfig.Realm != common.Config.Realm {
		// 更新realm
		iniConfig.Section("sip").Key("realm").SetValue(baseConfig.Realm)
		changed = true
	}

	if baseConfig.Port != common.Config.SipPort {
		// 更新端口
		iniConfig.Section("sip").Key("port").SetValue(strconv.Itoa(baseConfig.Port))
		sipChanged = true
	}

	if baseConfig.Host != common.Config.PublicIP {
		// 更新host
		iniConfig.Section("sip").Key("public_ip").SetValue(baseConfig.Host)
		sipChanged = true
	}

	if baseConfig.DevicePassword != common.Config.Password {
		// 更新设备密码
		iniConfig.Section("sip").Key("password").SetValue(baseConfig.DevicePassword)
		changed = true
	}

	// 更新优先流格式
	if baseConfig.PreferStreamFmt != "" && baseConfig.PreferStreamFmt != common.Config.PreferStreamFmt {
		iniConfig.Section("sip").Key("prefer_stream_fmt").SetValue(baseConfig.PreferStreamFmt)
		changed = true
	}

	// 更新订阅间隔
	if baseConfig.GlobalDeviceAlarmSubscribeInterval != common.Config.SubAlarmGlobalInterval {
		iniConfig.Section("sip").Key("sub_alarm_global_interval").SetValue(strconv.Itoa(baseConfig.GlobalDeviceAlarmSubscribeInterval))
		changed = true
	}

	if baseConfig.GlobalDeviceCatalogSubscribeInterval != common.Config.SubCatalogGlobalInterval {
		iniConfig.Section("sip").Key("sub_catalog_global_interval").SetValue(strconv.Itoa(baseConfig.GlobalDeviceCatalogSubscribeInterval))
		changed = true
	}

	if baseConfig.GlobalDevicePTZSubscribeInterval != common.Config.SubPTZGlobalInterval {
		iniConfig.Section("sip").Key("sub_ptz_global_interval").SetValue(strconv.Itoa(baseConfig.GlobalDevicePTZSubscribeInterval))
		changed = true
	}

	if baseConfig.GlobalDevicePositionSubscribeInterval != common.Config.SubPositionGlobalInterval {
		iniConfig.Section("sip").Key("sub_position_global_interval").SetValue(strconv.Itoa(baseConfig.GlobalDevicePositionSubscribeInterval))
		changed = true
	}

	// 更新默认媒体传输方式
	var setup = "udp"
	if strings.ToLower(baseConfig.MediaTransport) == "udp" {

	} else if strings.ToLower(baseConfig.MediaTransportMode) == "passive" {
		setup = "passive"
	} else if strings.ToLower(baseConfig.MediaTransportMode) == "active" {
		setup = "active"
	}

	if setup != common.Config.DeviceDefaultMediaTransport {
		iniConfig.Section("sip").Key("device_default_media_transport").SetValue(setup)
		changed = true
	}

	if !changed && !sipChanged {
		return "OK", nil
	}

	err = iniConfig.SaveTo("./config.ini")
	if err != nil {
		return nil, err
	}

	var newConfig *common.Config_
	newConfig, err = common.ParseConfig("./config.ini")
	if err != nil {
		return nil, err
	}

	// 重启sip服务
	if sipChanged {
		log.Sugar.Infof("重启sip服务器 port: %d", baseConfig.Port)
		if err = stack.RestartSipStack(newConfig); err != nil {
			return nil, err
		}
	}

	common.Config = newConfig
	return "OK", nil
}
