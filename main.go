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
	"github.com/go-co-op/gocron/v2"
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

	// 读取或生成密码MD5值
	hash := md5.Sum([]byte("admin"))
	AdminMD5 = hex.EncodeToString(hash[:])

	plaintext, md5Hex := ReadTempPwd()
	if plaintext != "" {
		log.Sugar.Infof("temp pwd: %s", plaintext)
	}

	PwdMD5 = md5Hex

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
		(&stack.Stream{StreamModel: stream}).Bye()
	}

	for _, sink := range sinks {
		(&stack.Sink{SinkModel: sink}).Close(true, false)
	}

	// 启动级联设备
	startPlatformDevices()
	// 启动1078设备
	startJTDevices()

	// 启动http服务
	httpAddr := net.JoinHostPort(config.ListenIP, strconv.Itoa(config.HttpPort))
	log.Sugar.Infof("启动http server. addr: %s", httpAddr)
	go startApiServer(httpAddr)

	// 启动目录刷新任务
	go stack.AddScheduledTask(time.Minute, true, stack.RefreshCatalogScheduleTask)
	// 启动订阅刷新任务
	go stack.AddScheduledTask(time.Minute, true, stack.RefreshSubscribeScheduleTask)

	// 启动定时任务, 每天凌晨3点执行
	s, _ := gocron.NewScheduler()
	defer func() { _ = s.Shutdown() }()

	_, _ = s.NewJob(
		gocron.CronJob(
			"0 3 * * *",
			false,
		),
		gocron.NewTask(
			func() {
				// 删除过期的位置、报警记录
				now := time.Now()
				alarmExpireTime := now.Add(time.Duration(common.Config.AlarmReserveDays) * 24 * time.Hour)
				positionExpireTime := now.Add(time.Duration(common.Config.PositionReserveDays) * 24 * time.Hour)
				// 删除过期的报警记录
				err := dao.Alarm.DeleteExpired(alarmExpireTime)
				if err != nil {
					log.Sugar.Errorf("删除过期的报警记录失败 err: %s", err.Error())
				}
				// 删除过期的位置记录
				err = dao.Position.DeleteExpired(positionExpireTime)
				if err != nil {
					log.Sugar.Errorf("删除过期的位置记录失败 err: %s", err.Error())
				}
			},
		),
	)

	s.Start()

	err = http.ListenAndServe(":19000", nil)
	if err != nil {
		println(err)
	}
}
