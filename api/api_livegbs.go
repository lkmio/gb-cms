package api

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/log"
	"gb-cms/stack"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	ModifyPasswordLock sync.Mutex
)

type LoginReq struct {
	Username   string `json:"username"`
	Pwd        string `json:"password"` // MD5加密
	RememberMe bool   `json:"remember_me"`
}

type ServerInfoBase struct {
	CopyrightText      string `json:"CopyrightText"`
	DemoUser           string `json:"DemoUser"`
	LiveStreamAuth     bool   `json:"LiveStreamAuth"`
	LoginRequestMethod string `json:"LoginRequestMethod"`
	LogoMiniText       string `json:"LogoMiniText"`
	LogoText           string `json:"LogoText"`

	MapInfo struct {
		Center  []float64 `json:"Center"`
		MaxZoom int       `json:"MaxZoom"`
		MinZoom int       `json:"MinZoom"`
		Zoom    int       `json:"Zoom"`
	} `json:"MapInfo"`
}

type ModifyPasswordReq struct {
	OldPwd string `json:"oldpassword"`
	NewPwd string `json:"newpassword"`
}

type StreamInfo struct {
	AudioEnable           bool   `json:"AudioEnable"`
	CDN                   string `json:"CDN"`
	CascadeSize           int    `json:"CascadeSize"`
	ChannelID             string `json:"ChannelID"`
	ChannelName           string `json:"ChannelName"`
	CloudRecord           bool   `json:"CloudRecord"`
	DecodeSize            int    `json:"DecodeSize"`
	DeviceID              string `json:"DeviceID"`
	Duration              int    `json:"Duration"`
	FLV                   string `json:"FLV"`
	HLS                   string `json:"HLS"`
	InBitRate             int    `json:"InBitRate"`
	InBytes               int    `json:"InBytes"`
	NumOutputs            int    `json:"NumOutputs"`
	Ondemand              bool   `json:"Ondemand"`
	OutBytes              int    `json:"OutBytes"`
	RTMP                  string `json:"RTMP"`
	RTPCount              int    `json:"RTPCount"`
	RTPLostCount          int    `json:"RTPLostCount"`
	RTPLostRate           int    `json:"RTPLostRate"`
	RTSP                  string `json:"RTSP"`
	RecordStartAt         string `json:"RecordStartAt"`
	RelaySize             int    `json:"RelaySize"`
	SMSID                 string `json:"SMSID"`
	SnapURL               string `json:"SnapURL"`
	SourceAudioCodecName  string `json:"SourceAudioCodecName"`
	SourceAudioSampleRate int    `json:"SourceAudioSampleRate"`
	SourceVideoCodecName  string `json:"SourceVideoCodecName"`
	SourceVideoFrameRate  int    `json:"SourceVideoFrameRate"`
	SourceVideoHeight     int    `json:"SourceVideoHeight"`
	SourceVideoWidth      int    `json:"SourceVideoWidth"`
	StartAt               string `json:"StartAt"`
	StreamID              string `json:"StreamID"`
	Transport             string `json:"Transport"`
	VideoFrameCount       int    `json:"VideoFrameCount"`
	WEBRTC                string `json:"WEBRTC"`
	WS_FLV                string `json:"WS_FLV"`
}

func GetUptime() time.Duration {
	return time.Since(StartUpTime)
}

func FormatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d Days %d Hours %d Mins %d Secs", days, hours, minutes, seconds)
}

func registerLiveGBSApi() {
	serverInfoBase := ServerInfoBase{
		CopyrightText:      fmt.Sprintf("Copyright © %d \u003ca href=\"//github.com/lkmio\" target=\"_blank\"\u003egithub.com/lkmio\u003c/a\u003e Released under MIT License", time.Now().Year()),
		DemoUser:           "",
		LiveStreamAuth:     true,
		LoginRequestMethod: "post",
		LogoMiniText:       "GBS",
		LogoText:           "LKMGBS",
		MapInfo: struct {
			Center  []float64 `json:"Center"`
			MaxZoom int       `json:"MaxZoom"`
			MinZoom int       `json:"MinZoom"`
			Zoom    int       `json:"Zoom"`
		}{
			Center:  []float64{0.0, 0.0},
			MaxZoom: 16,
			MinZoom: 8,
			Zoom:    12,
		},
	}

	apiServer.router.HandleFunc("/api/v1/login", common.WithFormDataParams(apiServer.OnLogin, LoginReq{}))
	apiServer.router.HandleFunc("/api/v1/modifypassword", withVerify(common.WithFormDataParams(apiServer.OnModifyPassword, ModifyPasswordReq{})))

	apiServer.router.HandleFunc("/api/v1/dashboard/auth", withVerify(func(writer http.ResponseWriter, request *http.Request) {
		response := struct {
			ChannelCount  int `json:"ChannelCount"`
			ChannelOnline int `json:"ChannelOnline"`
			ChannelTotal  int `json:"ChannelTotal"`
			DeviceOnline  int `json:"DeviceOnline"`
			DeviceTotal   int `json:"DeviceTotal"`
		}{
			ChannelCount:  16,
			ChannelOnline: ChannelOnlineCount,
			ChannelTotal:  ChannelTotalCount,
			DeviceOnline:  stack.OnlineDeviceManager.Count(),
			DeviceTotal:   DeviceCount,
		}

		_ = common.HttpResponseSuccess(writer, response)
	}))

	apiServer.router.HandleFunc("/api/v1/getserverinfo", withVerify2(func(writer http.ResponseWriter, request *http.Request) {
		response := struct {
			ServerInfoBase

			Authorization    string `json:"Authorization"`
			ChannelCount     int    `json:"ChannelCount"`
			Hardware         string `json:"Hardware"`
			InterfaceVersion string `json:"InterfaceVersion"`

			RemainDays  int    `json:"RemainDays"`
			RunningTime string `json:"RunningTime"`
			Server      string `json:"Server"`
			ServerTime  string `json:"ServerTime"`
			StartUpTime string `json:"StartUpTime"`
			VersionType string `json:"VersionType"`
		}{
			ServerInfoBase:   serverInfoBase,
			Authorization:    "Users",
			ChannelCount:     16,
			Hardware:         KernelArch,
			InterfaceVersion: "v1",

			RemainDays:  99,
			RunningTime: FormatUptime(GetUptime()),
			Server:      "github.com/lkmio/gb-cms dev",
			ServerTime:  time.Now().Format("2006-01-02 15:04:05"),
			StartUpTime: StartUpTime.Format("2006-01-02 15:04:05"),
			VersionType: "开源版",
		}

		_ = common.HttpResponseJson(writer, response)
	}, func(w http.ResponseWriter, req *http.Request) {
		_ = common.HttpResponseJson(w, &serverInfoBase)
	}))

	apiServer.router.HandleFunc("/api/v1/userinfo", withVerify(func(writer http.ResponseWriter, request *http.Request) {
		cookie, _ := request.Cookie("token")
		session := TokenManager.Find(cookie.Value)
		if session == nil {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}

		response := struct {
			Token         string   `json:"Token"`
			ID            int      `json:"ID"`
			Name          string   `json:"Name"`
			Roles         []string `json:"Roles"`
			HasAllChannel bool     `json:"HasAllChannel"`
			LoginAt       string   `json:"LoginAt"`
			RemoteIP      string   `json:"RemoteIP"`
			PwdModReq     bool     `json:"PwdModReq"`
		}{
			Token:         cookie.Value,
			ID:            1,
			Name:          "admin",
			Roles:         []string{"超级管理员"},
			HasAllChannel: true,
			LoginAt:       session.LoginTime.Format("2006-01-02 15:04:05"),
			RemoteIP:      request.RemoteAddr,
			PwdModReq:     AdminMD5 == PwdMD5,
		}

		_ = common.HttpResponseJson(writer, response)
	}))

	apiServer.router.HandleFunc("/api/v1/ispasswordchanged", func(writer http.ResponseWriter, request *http.Request) {
		_ = common.HttpResponseJson(writer, map[string]bool{
			"PasswordChanged": AdminMD5 != PwdMD5,
			"UserChanged":     false,
		})
	})

	apiServer.router.HandleFunc("api/v1/dashboard/auth", withVerify(func(writer http.ResponseWriter, request *http.Request) {

	}))

	apiServer.router.HandleFunc("/api/v1/dashboard/top", withVerify(func(writer http.ResponseWriter, request *http.Request) {
		_ = common.HttpResponseJsonStr(writer, topStatsJson)
	}))

	// 实时统计上下行流量
	apiServer.router.HandleFunc("/api/v1/dashboard/top/net", withVerify(func(writer http.ResponseWriter, request *http.Request) {
		_ = common.HttpResponseJsonStr(writer, lastNetStatsJson)
	}))

	apiServer.router.HandleFunc("/api/v1/dashboard/store", withVerify(func(writer http.ResponseWriter, request *http.Request) {
		_ = common.HttpResponseJsonStr(writer, diskStatsJson)
	}))
}

func (api *ApiServer) OnLogin(v *LoginReq, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if PwdMD5 != v.Pwd {
		log.Sugar.Errorf("登录失败, 密码错误 pwd: %s remote addr: %s", v.Pwd, r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("用户名或密码错误"))
		return nil, nil
	}

	token := GenerateToken()
	TokenManager.Add(token, v.Username, v.Pwd)

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})

	response := struct {
		AuthToken    string
		CookieToken  string
		Token        string
		TokenTimeout int
		URLToken     string
	}{
		AuthToken:    token,
		CookieToken:  token,
		Token:        token,
		TokenTimeout: 0,
		URLToken:     token,
	}

	return response, nil
}

func (api *ApiServer) OnModifyPassword(v *ModifyPasswordReq, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	ModifyPasswordLock.Lock()
	defer ModifyPasswordLock.Unlock()
	// 如果是首次修改密码, livegbs前端旧密码携带的是空密码, 所以首次修改不检验旧密码
	if AdminMD5 != PwdMD5 && PwdMD5 != v.OldPwd {
		log.Sugar.Errorf("修改密码失败, 旧密码错误 oldPwd: %s remote addr: %s", v.OldPwd, r.RemoteAddr)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("原密码不正确"))
		return nil, nil
	}

	// 写入新密码
	err := os.WriteFile("./data/pwd.txt", []byte(v.NewPwd), 0644)
	if err != nil {
		log.Sugar.Errorf("修改密码失败, 写入文件失败 err: %s pwd: %s", err.Error(), v.NewPwd)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("系统错误"))
		return nil, nil
	}

	// 删除所有token?
	TokenManager.Clear()
	PwdMD5 = v.NewPwd
	return nil, nil
}
