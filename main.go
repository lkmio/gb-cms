package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"gb-cms/api"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/hook"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/pretty66/websocketproxy"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"
)

func init() {
	api.StartUpTime = time.Now()

	logConfig := common.LogConfig{
		Level:     int(zapcore.DebugLevel),
		Name:      "./logs/lkm_gb_cms",
		MaxSize:   10,
		MaxBackup: 100,
		MaxAge:    7,
		Compress:  false,
	}

	log.InitLogger(zapcore.Level(logConfig.Level), logConfig.Name, logConfig.MaxSize, logConfig.MaxBackup, logConfig.MaxAge, logConfig.Compress)
	websocketproxy.SetLogger(zap.NewStdLog(log.Sugar.Desugar()))
}

func main() {
	config, err := common.ParseConfig("./config.ini")
	if err != nil {
		panic(err)
	}

	common.Config = config
	indent, _ := json.MarshalIndent(common.Config, "", "\t")
	log.Sugar.Infof("server config:\r\n%s", indent)

	if config.Hooks.OnInvite != "" {
		hook.RegisterEventUrl(hook.EventTypeDeviceOnInvite, config.Hooks.OnInvite)
	}

	// 读取或生成密码MD5值
	hash := md5.Sum([]byte("admin"))
	api.AdminMD5 = hex.EncodeToString(hash[:])

	plaintext, md5Hex := api.ReadTempPwd()
	if plaintext != "" {
		log.Sugar.Infof("temp pwd: %s", plaintext)
	}

	api.PwdMD5 = md5Hex

	// 加载黑名单
	blacklists, err := dao.Blacklist.Load()
	if err != nil {
		log.Sugar.Errorf("加载黑名单失败 err: %s", err.Error())
	} else {
		for _, blacklist := range blacklists {
			if blacklist.Rule == "ip" {
				_ = dao.BlacklistManager.SaveIP(blacklist.Key)
			} else if blacklist.Rule == "ua" {
				_ = dao.BlacklistManager.SaveUA(blacklist.Key)
			}
		}
	}

	// 启动web session超时管理
	go api.TokenManager.Start(5 * time.Minute)

	// 启动sip server
	sipServer := &stack.SipServer{}
	err = sipServer.Start(config.SipID, config.ListenIP, config.PublicIP, config.SipPort)
	if err != nil {
		panic(err)
	}

	log.Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	common.Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	common.SipStack = sipServer

	stack.Start()

	go api.StartStats()

	// 启动http服务
	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	log.Sugar.Infof("启动http server. addr: %s", httpAddr)
	go api.StartApiServer(httpAddr)

	err = http.ListenAndServe(":19000", nil)
	if err != nil {
		println(err)
	}
}
