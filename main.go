package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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

var (
	AdminMD5    string // 明文密码"admin"的MD5值
	PwdMD5      string
	StartUpTime time.Time
)

func init() {
	StartUpTime = time.Now()

	logConfig := common.LogConfig{
		Level:     int(zapcore.DebugLevel),
		Name:      "./logs/clog",
		MaxSize:   10,
		MaxBackup: 100,
		MaxAge:    7,
		Compress:  false,
	}

	log.InitLogger(zapcore.Level(logConfig.Level), logConfig.Name, logConfig.MaxSize, logConfig.MaxBackup, logConfig.MaxAge, logConfig.Compress)
	websocketproxy.SetLogger(zap.NewStdLog(log.Sugar.Desugar()))
}

func main() {
	config, err := common.ParseConfig("./config.json")
	if err != nil {
		panic(err)
	}

	common.Config = config
	indent, _ := json.MarshalIndent(common.Config, "", "\t")
	log.Sugar.Infof("server config:\r\n%s", indent)

	if config.Hooks.OnInvite != "" {
		hook.RegisterEventUrl(hook.EventTypeDeviceOnInvite, config.Hooks.OnInvite)
	}

	hash := md5.Sum([]byte("admin"))
	AdminMD5 = hex.EncodeToString(hash[:])

	plaintext, md5 := ReadTempPwd()
	if plaintext != "" {
		log.Sugar.Infof("temp pwd: %s", plaintext)
	}

	PwdMD5 = md5

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

	// 启动session超时管理
	go TokenManager.Start(5 * time.Minute)

	// 启动设备在线超时管理
	stack.OnlineDeviceManager.Start(time.Duration(common.Config.AliveExpires)*time.Second/4, time.Duration(common.Config.AliveExpires)*time.Second, stack.OnExpires)

	// 从数据库中恢复会话
	var streams map[string]*dao.StreamModel
	var sinks map[string]*dao.SinkModel

	// 查询在线设备, 更新设备在线状态
	updateDevicesStatus()

	// 恢复国标推流会话
	streams, sinks = recoverStreams()

	// 启动sip server
	server, err := stack.StartSipServer(config.SipID, config.ListenIP, config.PublicIP, config.SipPort)
	if err != nil {
		panic(err)
	}

	go StartStats()

	log.Sugar.Infof("启动sip server成功. addr: %s:%d", config.ListenIP, config.SipPort)
	common.Config.SipContactAddr = net.JoinHostPort(config.PublicIP, strconv.Itoa(config.SipPort))
	common.SipStack = server

	// 在sip启动后, 关闭无效的流
	for _, stream := range streams {
		(&stack.Stream{stream}).Bye()
	}

	for _, sink := range sinks {
		(&stack.Sink{sink}).Close(true, false)
	}

	// 启动级联设备
	startPlatformDevices()
	// 启动1078设备
	startJTDevices()

	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	log.Sugar.Infof("启动http server. addr: %s", httpAddr)
	go startApiServer(httpAddr)

	// 启动目录刷新任务
	stack.AddScheduledTask(time.Minute, true, stack.RefreshCatalogScheduleTask)

	err = http.ListenAndServe(":19000", nil)
	if err != nil {
		println(err)
	}
}
