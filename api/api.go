package api

import (
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type ApiServer struct {
	router      *mux.Router
	upgrader    *websocket.Upgrader
	actionNames map[string]string
}

type InviteParams struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
	Setup     string `json:"setup"`
	Speed     string `json:"speed"`
	Token     string `json:"token"`
	Download  bool   `json:"download"`
	streamId  common.StreamID
}

type StreamParams struct {
	Stream     common.StreamID `json:"stream"`      // Source
	Protocol   int             `json:"protocol"`    // 推拉流协议
	RemoteAddr string          `json:"remote_addr"` // peer地址
}

type PlayDoneParams struct {
	StreamParams
	Sink string `json:"sink"`
}

type QueryRecordParams struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	Timeout   int    `json:"timeout"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
	//Type_     string `json:"type"`
	Command string `json:"command"` // 云台控制命令 left/up/right/down/zoomin/zoomout
}

type DeviceChannelID struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

type SeekParams struct {
	StreamId common.StreamID `json:"stream_id"`
	Seconds  int             `json:"seconds"`
}

type BroadcastParams struct {
	DeviceID  string            `json:"device_id"`
	ChannelID string            `json:"channel_id"`
	StreamId  common.StreamID   `json:"stream_id"`
	Setup     *common.SetupType `json:"setup"`
}

type RecordParams struct {
	StreamParams
	Path string `json:"path"`
}

type StreamIDParams struct {
	StreamID common.StreamID `json:"streamid"`
	Command  string          `json:"command"`
	Scale    float64         `json:"scale"`
}

type PageQuery struct {
	PageNumber *int        `json:"page_number"` // 页数
	PageSize   *int        `json:"page_size"`   // 每页大小
	TotalPages int         `json:"total_pages"` // 总页数
	TotalCount int         `json:"total_count"` // 总记录数
	Data       interface{} `json:"data"`
}

type SetMediaTransportReq struct {
	DeviceID           string `json:"serial"`
	MediaTransport     string `json:"media_transport"`
	MediaTransportMode string `json:"media_transport_mode"`
}

// QueryDeviceChannel 查询设备和通道的参数
type QueryDeviceChannel struct {
	DeviceID    string `json:"serial"`
	GroupID     string `json:"dir_serial"`
	PCode       string `json:"pcode"`
	Start       int    `json:"start"`
	Limit       int    `json:"limit"`
	Keyword     string `json:"q"`
	Online      string `json:"online"`
	Enable      string `json:"enable"`
	ChannelType string `json:"channel_type"` // dir-查询子目录
	Order       string `json:"order"`        // asc/desc
	Sort        string `json:"sort"`         // Channel-根据数据库ID排序/iD-根据通道ID排序
	SMS         string `json:"sms"`
	Filter      string `json:"filter"`

	Priority  int    `json:"priority"` // 报警参数
	Method    string `json:"method"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
}

type DeleteDevice struct {
	DeviceID string `json:"serial"`
	IP       string `json:"ip"`
	Forbid   bool   `json:"forbid"`
	UA       string `json:"ua"`
}

type SetEnable struct {
	ID              int  `json:"id"`
	Enable          bool `json:"enable"`
	ShareAllChannel bool `json:"shareallchannel"`
}

type QueryCascadeChannelList struct {
	QueryDeviceChannel
	ID      string `json:"id"`
	Related bool   `json:"related"` // 只看已选择
	Reverse bool   `json:"reverse"`
}

type ChannelListResult struct {
	ChannelCount int               `json:"ChannelCount"`
	ChannelList  []*LiveGBSChannel `json:"ChannelList"`
}

type CascadeChannel struct {
	CascadeID string
	*LiveGBSChannel
}

type CustomChannel struct {
	DeviceID  string `json:"serial"`
	ChannelID string `json:"code"`
	CustomID  string `json:"id"`
}

type DeviceInfo struct {
	DeviceID           string  `json:"serial"`
	CustomName         string  `json:"custom_name"`
	MediaTransport     string  `json:"media_transport"`
	MediaTransportMode string  `json:"media_transport_mode"`
	StreamMode         string  `json:"stream_mode"`
	SMSID              string  `json:"sms_id"`
	SMSGroupID         string  `json:"sms_group_id"`
	RecvStreamIP       string  `json:"recv_stream_ip"`
	ContactIP          string  `json:"contact_ip"`
	Charset            string  `json:"charset"`
	CatalogInterval    int     `json:"catalog_interval"`
	SubscribeInterval  int     `json:"subscribe_interval"`
	CatalogSubscribe   bool    `json:"catalog_subscribe"`
	AlarmSubscribe     bool    `json:"alarm_subscribe"`
	PositionSubscribe  bool    `json:"position_subscribe"`
	PTZSubscribe       bool    `json:"ptz_subscribe"`
	RecordCenter       bool    `json:"record_center"`
	RecordIndistinct   bool    `json:"record_indistinct"`
	CivilCodeFirst     bool    `json:"civil_code_first"`
	KeepOriginalTree   bool    `json:"keep_original_tree"`
	Password           string  `json:"password"`
	DropChannelType    string  `json:"drop_channel_type"`
	Longitude          float64 `json:"longitude"`
	Latitude           float64 `json:"latitude"`
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return r.ResponseWriter.Write(b)
}

type Empty struct {
}

var apiServer *ApiServer

func init() {
	apiServer = &ApiServer{
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},

		actionNames: make(map[string]string, 32),
		router:      mux.NewRouter(),
	}
}

// 验证和刷新token
func withVerify(f func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ok := TokenManager.Refresh(cookie.Value, time.Now())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else if AdminMD5 == PwdMD5 && req.URL.Path != "/api/v1/modifypassword" && req.URL.Path != "/api/v1/userinfo" {
			// 如果没有修改默认密码, 只允许放行这2个接口
			return
		}

		f(w, req)
	}
}

func withVerify2(onSuccess func(w http.ResponseWriter, req *http.Request), onFailure func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err == nil && TokenManager.Refresh(cookie.Value, time.Now()) {
			onSuccess(w, req)
		} else {
			onFailure(w, req)
		}
	}
}

// 注册带统计的api
func (api *ApiServer) registerStatisticsHandler(actionName, path string, handler func(http.ResponseWriter, *http.Request)) {
	api.router.HandleFunc(path, handler)
	api.actionNames[path] = actionName
}

func StartApiServer(addr string) {
	apiServer.router.HandleFunc("/api/v1/hook/on_play", common.WithJsonParams(apiServer.OnPlay, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_play_done", common.WithJsonParams(apiServer.OnPlayDone, &PlayDoneParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish", common.WithJsonParams(apiServer.OnPublish, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_publish_done", common.WithJsonParams(apiServer.OnPublishDone, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_idle_timeout", common.WithJsonParams(apiServer.OnIdleTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_receive_timeout", common.WithJsonParams(apiServer.OnReceiveTimeout, &StreamParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_record", common.WithJsonParams(apiServer.OnRecord, &RecordParams{}))
	apiServer.router.HandleFunc("/api/v1/hook/on_started", apiServer.OnStarted)

	apiServer.registerStatisticsHandler("开始预览", "/api/v1/stream/start", withVerify(common.WithFormDataParams(apiServer.OnStreamStart, InviteParams{})))           // 实时预览
	apiServer.registerStatisticsHandler("停止预览", "/api/v1/stream/stop", withVerify(common.WithFormDataParams(apiServer.OnCloseLiveStream, InviteParams{})))        // 关闭实时预览
	apiServer.registerStatisticsHandler("开始回放/下载", "/api/v1/playback/start", withVerify(common.WithFormDataParams(apiServer.OnPlaybackStart, InviteParams{})))    // 回放/下载
	apiServer.registerStatisticsHandler("停止回放/下载", "/api/v1/playback/stop", withVerify(common.WithFormDataParams(apiServer.OnCloseStream, StreamIDParams{})))     // 关闭回放/下载
	apiServer.registerStatisticsHandler("回放控制", "/api/v1/playback/control", withVerify(common.WithFormDataParams(apiServer.OnPlaybackControl, StreamIDParams{}))) // 回放控制

	apiServer.router.HandleFunc("/api/v1/device/list", withVerify(common.WithQueryStringParams(apiServer.OnDeviceList, QueryDeviceChannel{})))                                                // 查询设备列表
	apiServer.router.HandleFunc("/api/v1/device/channeltree", withVerify(common.WithQueryStringParams(apiServer.OnDeviceTree, QueryDeviceChannel{})))                                         // 设备树
	apiServer.router.HandleFunc("/api/v1/device/channellist", withVerify(common.WithQueryStringParams(apiServer.OnChannelList, QueryDeviceChannel{})))                                        // 查询通道列表
	apiServer.registerStatisticsHandler("手动刷新通道", "/api/v1/device/fetchcatalog", withVerify(common.WithQueryStringParams(apiServer.OnCatalogQuery, QueryDeviceChannel{})))                    // 更新通道
	apiServer.registerStatisticsHandler("删除设备", "/api/v1/device/remove", withVerify(common.WithFormDataParams(apiServer.OnDeviceRemove, DeleteDevice{})))                                     // 删除设备
	apiServer.registerStatisticsHandler("设置设备媒体传输模式", "/api/v1/device/setmediatransport", withVerify(common.WithFormDataParams(apiServer.OnDeviceMediaTransportSet, SetMediaTransportReq{}))) // 设置设备媒体传输模式

	apiServer.registerStatisticsHandler("查询录像列表", "/api/v1/playback/recordlist", withVerify(common.WithQueryStringParams(apiServer.OnRecordList, QueryRecordParams{}))) // 查询录像列表
	apiServer.router.HandleFunc("/api/v1/stream/info", withVerify(apiServer.OnStreamInfo))
	apiServer.router.HandleFunc("/api/v1/playback/streaminfo", withVerify(apiServer.OnStreamInfo))
	apiServer.router.HandleFunc("/api/v1/device/session/list", withVerify(common.WithQueryStringParams(apiServer.OnSessionList, QueryDeviceChannel{}))) // 推流列表
	apiServer.router.HandleFunc("/api/v1/device/session/stop", withVerify(common.WithFormDataParams(apiServer.OnSessionStop, StreamIDParams{})))        // 关闭流
	apiServer.router.HandleFunc("/api/v1/device/setchannelid", withVerify(common.WithFormDataParams(apiServer.OnCustomChannelSet, CustomChannel{})))    // 自定义通道ID

	apiServer.router.HandleFunc("/api/v1/playback/seek", common.WithJsonResponse(apiServer.OnSeekPlayback, &SeekParams{}))                                 // 回放seek
	apiServer.registerStatisticsHandler("云台控制", "/api/v1/control/ptz", withVerify(common.WithFormDataParams(apiServer.OnPTZControl, QueryRecordParams{}))) // 云台控制

	apiServer.router.HandleFunc("/api/v1/cascade/list", withVerify(common.WithQueryStringParams(apiServer.OnPlatformList, QueryDeviceChannel{})))                    // 级联设备列表
	apiServer.registerStatisticsHandler("添加级联设备", "/api/v1/cascade/save", withVerify(common.WithFormDataParams(apiServer.OnPlatformAdd, LiveGBSCascade{})))          // 添加级联设备
	apiServer.registerStatisticsHandler("设置级联设备状态", "/api/v1/cascade/setenable", withVerify(common.WithFormDataParams(apiServer.OnEnableSet, SetEnable{})))          // 使能级联设备
	apiServer.registerStatisticsHandler("删除级联设备", "/api/v1/cascade/remove", withVerify(common.WithFormDataParams(apiServer.OnPlatformRemove, SetEnable{})))          // 删除级联设备
	apiServer.router.HandleFunc("/api/v1/cascade/channellist", withVerify(common.WithQueryStringParams(apiServer.OnPlatformChannelList, QueryCascadeChannelList{}))) // 级联设备通道列表

	apiServer.router.HandleFunc("/api/v1/cascade/savechannels", withVerify(apiServer.OnPlatformChannelBind))                                                // 级联绑定通道
	apiServer.router.HandleFunc("/api/v1/cascade/removechannels", withVerify(apiServer.OnPlatformChannelUnbind))                                            // 级联解绑通道
	apiServer.router.HandleFunc("/api/v1/cascade/setshareallchannel", withVerify(common.WithFormDataParams(apiServer.OnShareAllChannel, SetEnable{})))      // 开启或取消级联所有通道
	apiServer.registerStatisticsHandler("推送目录", "/api/v1/cascade/pushcatalog", withVerify(common.WithFormDataParams(apiServer.OnCatalogPush, SetEnable{}))) // 推送目录
	apiServer.registerStatisticsHandler("编辑设备信息", "/api/v1/device/setinfo", withVerify(common.WithFormDataParams(apiServer.OnDeviceInfoSet, DeviceInfo{}))) // 编辑设备信息
	apiServer.router.HandleFunc("/api/v1/alarm/list", withVerify(common.WithQueryStringParams(apiServer.OnAlarmList, QueryDeviceChannel{})))                // 报警查询
	apiServer.registerStatisticsHandler("删除报警", "/api/v1/alarm/remove", withVerify(common.WithFormDataParams(apiServer.OnAlarmRemove, SetEnable{})))        // 删除报警
	apiServer.registerStatisticsHandler("清空报警", "/api/v1/alarm/clear", withVerify(common.WithFormDataParams(apiServer.OnAlarmClear, Empty{})))              // 清空报警
	apiServer.router.HandleFunc("/api/v1/log/list", withVerify(common.WithQueryStringParams(apiServer.OnLogList, QueryDeviceChannel{})))                    // 操作日志
	apiServer.router.HandleFunc("/api/v1/log/clear", withVerify(common.WithQueryStringParams(apiServer.OnLogClear, Empty{})))                               // 操作日志

	apiServer.router.HandleFunc("/api/v1/device/statuslog", withVerify(common.WithQueryStringParams(apiServer.OnStatusLogList, QueryDeviceChannel{}))) // 设备上下线统计

	// 暂未开发
	apiServer.router.HandleFunc("/api/v1/sms/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))                  // 流媒体服务器列表
	apiServer.router.HandleFunc("/api/v1/cloudrecord/querychannels", withVerify(func(w http.ResponseWriter, req *http.Request) {})) // 云端录像
	apiServer.router.HandleFunc("/api/v1/user/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))                 // 用户管理
	apiServer.router.HandleFunc("/api/v1/getbaseconfig", withVerify(common.WithFormDataParams(apiServer.OnGetBaseConfig, Empty{})))
	apiServer.router.HandleFunc("/api/v1/setbaseconfig", withVerify(common.WithFormDataParams(apiServer.OnSetBaseConfig, Empty{})))
	apiServer.router.HandleFunc("/api/v1/gm/cert/list", withVerify(func(w http.ResponseWriter, req *http.Request) {}))
	apiServer.router.HandleFunc("/api/v1/getrequestkey", withVerify(func(w http.ResponseWriter, req *http.Request) {}))
	apiServer.router.HandleFunc("/api/v1/getrequestkey", withVerify(func(w http.ResponseWriter, req *http.Request) {}))
	apiServer.router.HandleFunc("/api/v1/device/positionlog", withVerify(func(w http.ResponseWriter, req *http.Request) {}))
	apiServer.router.HandleFunc("/api/v1/device/streamlog", withVerify(func(w http.ResponseWriter, req *http.Request) {}))

	apiServer.registerStatisticsHandler("开始录制", "/api/v1/record/start", withVerify(apiServer.OnRecordStart)) // 开启录制
	apiServer.registerStatisticsHandler("结束录制", "/api/v1/record/stop", withVerify(apiServer.OnRecordStop))   // 关闭录制

	apiServer.router.HandleFunc("/api/v1/broadcast/invite", common.WithJsonResponse(apiServer.OnBroadcast, &BroadcastParams{Setup: &common.DefaultSetupType})) // 发起语音广播
	apiServer.router.HandleFunc("/api/v1/broadcast/hangup", common.WithJsonResponse(apiServer.OnHangup, &BroadcastParams{}))                                   // 挂断广播会话
	apiServer.registerStatisticsHandler("发起对讲", "/api/v1/control/ws-talk/{device}/{channel}", withVerify(apiServer.OnTalk))                                    // 一对一语音对讲

	apiServer.router.HandleFunc("/api/v1/jt/device/add", common.WithJsonResponse(apiServer.OnVirtualDeviceAdd, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/edit", common.WithJsonResponse(apiServer.OnVirtualDeviceEdit, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/remove", common.WithJsonResponse(apiServer.OnVirtualDeviceRemove, &dao.JTDeviceModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/device/list", common.WithJsonResponse(apiServer.OnVirtualDeviceList, &PageQuery{}))

	apiServer.router.HandleFunc("/api/v1/jt/channel/add", common.WithJsonResponse(apiServer.OnVirtualChannelAdd, &dao.ChannelModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/edit", common.WithJsonResponse(apiServer.OnVirtualChannelEdit, &dao.ChannelModel{}))
	apiServer.router.HandleFunc("/api/v1/jt/channel/remove", common.WithJsonResponse(apiServer.OnVirtualChannelRemove, &dao.ChannelModel{}))
	apiServer.registerStatisticsHandler("退出登录", "/api/v1/logout", func(writer http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err == nil {
			TokenManager.Remove(cookie.Value)
			writer.Header().Set("Location", "/login.html")
			writer.WriteHeader(http.StatusFound)
			return
		}
	})

	registerLiveGBSApi()

	apiServer.router.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			name, ok := apiServer.actionNames[r.URL.Path]
			recorder := &responseRecorder{ResponseWriter: w}
			handler.ServeHTTP(recorder, r)
			if !ok {
				return
			}

			end := time.Now()
			var address string
			if common.Config.IP2RegionEnable {
				host, _, _ := net.SplitHostPort(r.RemoteAddr)
				region, err := common.IP2Region(host)
				if err == nil {
					address = region
				} else {
					log.Sugar.Errorf("IP2Region error: %v", err)
				}
			}

			// /api/v1/login
			var username = "admin"
			if r.URL.Path == "/api/v1/login" {
				username = r.FormValue("username")
			}

			var status = "OK"
			if recorder.statusCode == 0 {
				recorder.statusCode = http.StatusOK
			}

			if recorder.statusCode != http.StatusOK {
				status = string(recorder.body)
			}

			model := dao.LogModel{
				Name:         name,
				Scheme:       "HTTP",
				Method:       r.Method,
				RequestURI:   r.URL.Path,
				RemoteAddr:   r.RemoteAddr,
				RemoteRegion: address,
				Status:       status,
				StatusCode:   recorder.statusCode,
				StartAt:      now.Format("2006-01-02 15:04:05"),
				Duration:     int(end.Sub(now).Seconds()),
				Username:     username,
			}

			err := dao.Log.Save(&model)
			if err != nil {
				log.Sugar.Errorf("Save log error: %v", err)
			}
		})
	})

	// 前端路由
	htmlRoot := "./html/"
	fileServer := http.FileServer(http.Dir(htmlRoot))
	apiServer.router.PathPrefix("/").HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// 处理无扩展名的路径，自动添加.html扩展名
		path := request.URL.Path
		if !strings.Contains(path, ".") {
			// 检查是否存在对应的.html文件
			htmlPath := htmlRoot + path + ".html"
			if _, err := os.Stat(htmlPath); err == nil {
				// 如果存在对应的.html文件，则直接返回该文件
				http.ServeFile(writer, request, htmlPath)
				return
			}
		}

		fileServer.ServeHTTP(writer, request)
	})

	srv := &http.Server{
		Handler: apiServer.router,
		Addr:    addr,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	err := srv.ListenAndServe()

	if err != nil {
		panic(err)
	}
}
