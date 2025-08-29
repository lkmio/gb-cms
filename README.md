## JT1078转GB28181流程

### 1. 创建GB28181 UA
调用接口: http://localhost:9000/api/v1/jt/device/add 请求体如下：

```
{
    "username": "34020000001400000001",
    "server_id": "34020000002000000001",
    "server_addr": "192.168.31.112:15060",
    "transport": "udp",
    "password": "12345678",
    "register_expires": 3600,
    "keepalive_interval": 60,
    "name": "模拟1078设备4",
    "sim_number":"13800138000",
    "manufacturer":"github.com/lkmio",
    "model":"gb-cms",
    "firmware":"dev"
}

```

username: 自定义国标设备ID, 唯一键

sim_number: 对应的部标设备sim卡号, 唯一键


### 2. 添加视频通道

调用接口: http://localhost:9000/api/v1/jt/channel/add 请求体如下：

```
{
    "root_id": "34020000001400000001",
    "device_id": "34020000001310000001",
    "name": "视频通道",
    "manufacturer": "github.com/lkmio",
    "model": "gb-cms",
    "owner": "github.com/lkmio",
    "channel_number": 1
}

```

root_id: 创建GB28181 UA接口的username

device_id: 自定义国标视频通道ID

channel_number: 国标视频通道ID对应的1078视频通道号

### 3. 实现invite钩子

```
{
  "sip_port": 5060,
  "http_port": 9000,
  "listen_ip": "0.0.0.0",
  "public_ip": "192.168.31.112",

  "sip_id":"34020000002000000001",
  "password":"123456",
  "alive_expires": 180,
  "mobile_position_interval": 10,

  "media_server": "0.0.0.0:8080",

  "?auto_close_on_idle": "拉流空闲时, 立即关闭流",
  "auto_close_on_idle": true,

  "hooks": {
    "?online": "设备上线通知",
    "online": "",

    "?offline": "设备下线通知",
    "offline": "",

    "?position" : "设备位置通知",
    "position": "",

    "?on_invite": "被邀请, 用于通知1078信令服务器, 向设备下发推流指令",
    "on_invite": "http://localhost:8081/api/v1/jt1078/on_invite",

    "?on_answer": "被查询录像,用于通知1078信令服务器",
    "on_query_record": ""
  }
}

```

用户自行实现`on_invite`钩子, 当上级国标服务器预览部标设备时, 会通过`on_invite`钩子, 通知部标信令服务器。此时部标信令服务器, 向设备下发请求视频信令，推流到lkm收流端口, lkm再转发到国标流媒体服务器。[钩子示例如下](https://github.com/lkmio/lkm/blob/02689f5e09b1f2ffccf26aad62e7b930e30aeafe/jt1078/jt_test.go#L210): 

```
	t.Run("hook_on_invite", func(t *testing.T) {
		// 创建http server
		router := mux.NewRouter()

		// 示例路由
		router.HandleFunc("/api/v1/jt1078/on_invite", func(w http.ResponseWriter, r *http.Request) {
			v := struct {
				SimNumber     string `json:"sim_number,omitempty"`
				ChannelNumber string `json:"channel_number,omitempty"`
			}{}

			// 读取请求体
			bytes := make([]byte, 1024)
			n, err := r.Body.Read(bytes)
			if n < 1 {
				panic(err)
			}
			err = json.Unmarshal(bytes[:n], &v)
			if err != nil {
				panic(err)
			}

			fmt.Printf("on_invite sim_number: %s, channel_number: %s\r\n", v.SimNumber, v.ChannelNumber)
			w.WriteHeader(http.StatusOK)
			go publish()
		})

		server := &http.Server{
			Addr:         "localhost:8081",
			Handler:      router,
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}

		err := server.ListenAndServe()
		if err != nil {
			panic(err)
		}
	})
```
