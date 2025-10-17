package api

import (
	"errors"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"net"
	"net/http"
	"strconv"
)

func (api *ApiServer) OnPlatformAdd(v *LiveGBSCascade, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("添加级联设备 %v", *v)

	if v.Username == "" {
		v.Username = common.Config.SipID
		log.Sugar.Infof("级联设备使用本级域: %s", common.Config.SipID)
	}

	var err error
	if len(v.Username) != 20 {
		err = fmt.Errorf("用户名长度必须20位")
		return nil, err
	} else if len(v.Serial) != 20 {
		err = fmt.Errorf("上级ID长度必须20位")
		return nil, err
	}

	if err != nil {
		log.Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	}

	v.Status = "OFF"
	model := dao.PlatformModel{
		SIPUAOptions: common.SIPUAOptions{
			Name:              v.Name,
			Username:          v.Username,
			Password:          v.Password,
			ServerID:          v.Serial,
			ServerAddr:        net.JoinHostPort(v.Host, strconv.Itoa(v.Port)),
			Transport:         v.CommandTransport,
			RegisterExpires:   v.RegisterInterval,
			KeepaliveInterval: v.KeepaliveInterval,
			Status:            common.OFF,
		},

		Enable: v.Enable,
	}

	platform, err := stack.NewPlatform(&model.SIPUAOptions, common.SipStack)
	if err != nil {
		return nil, err
	}

	// 编辑国标设备
	if v.ID != "" {
		// 停止旧的
		oldPlatform := stack.PlatformManager.Remove(model.ServerAddr)
		if oldPlatform != nil {
			oldPlatform.Stop()
		}

		// 更新数据库
		id, _ := strconv.ParseInt(v.ID, 10, 64)
		model.ID = uint(id)
		err = dao.Platform.UpdatePlatform(&model)
	} else {
		err = dao.Platform.SavePlatform(&model)
	}

	if err == nil && v.Enable {
		if !stack.PlatformManager.Add(model.ServerAddr, platform) {
			err = fmt.Errorf("地址冲突. key: %s", model.ServerAddr)
			if err != nil {
				_ = dao.Platform.DeletePlatformByAddr(model.ServerAddr)
			}
		} else {
			platform.Start()
		}
	}

	if err != nil {
		log.Sugar.Errorf("添加级联设备失败 err: %s", err.Error())
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformRemove(v *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("删除级联设备 %v", *v)
	platform, _ := dao.Platform.QueryPlatformByID(v.ID)
	if platform == nil {
		return nil, fmt.Errorf("级联设备不存在")
	}

	_ = dao.Platform.DeletePlatformByID(v.ID)
	client := stack.PlatformManager.Remove(platform.ServerAddr)
	if client != nil {
		client.Stop()
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformList(q *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	// 分页参数
	if q.Limit < 1 {
		q.Limit = 10
	}

	response := struct {
		CascadeCount int               `json:"CascadeCount"`
		CascadeList  []*LiveGBSCascade `json:"CascadeList"`
	}{}

	platforms, total, err := dao.Platform.QueryPlatforms((q.Start/q.Limit)+1, q.Limit, q.Keyword, q.Enable, q.Online)
	if err == nil {
		response.CascadeCount = total
		for _, platform := range platforms {
			host, p, _ := net.SplitHostPort(platform.ServerAddr)
			port, _ := strconv.Atoi(p)
			response.CascadeList = append(response.CascadeList, &LiveGBSCascade{
				ID:                strconv.Itoa(int(platform.ID)),
				Enable:            platform.Enable,
				Name:              platform.Name,
				Serial:            platform.ServerID,
				Realm:             platform.ServerID[:10],
				Host:              host,
				Port:              port,
				LocalSerial:       platform.Username,
				Username:          platform.Username,
				Password:          platform.Password,
				Online:            platform.Status == common.ON,
				Status:            platform.Status,
				RegisterInterval:  platform.RegisterExpires,
				KeepaliveInterval: platform.KeepaliveInterval,
				CommandTransport:  platform.Transport,
				Charset:           "GB2312",
				CatalogGroupSize:  1,
				LoadLimit:         0,
				CivilCodeLimit:    8,
				DigestAlgorithm:   "",
				GM:                false,
				Cert:              "***",
				CreatedAt:         platform.CreatedAt.Format("2006-01-02 15:04:05"),
				UpdatedAt:         platform.UpdatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return response, nil
}

func (api *ApiServer) OnPlatformChannelBind(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	channels := r.Form["channels[]"]

	var err error
	id, _ := strconv.Atoi(idStr)
	_, err = dao.Platform.QueryPlatformByID(id)
	if err == nil {
		err = dao.Platform.BindChannels(id, channels)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, err.Error())
	} else {
		_ = common.HttpResponseJson(w, "OK")
	}
}

func (api *ApiServer) OnPlatformChannelUnbind(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	channels := r.Form["channels[]"]

	var err error
	id, _ := strconv.Atoi(idStr)
	_, err = dao.Platform.QueryPlatformByID(id)
	if err == nil {
		err = dao.Platform.UnbindChannels(id, channels)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, err.Error())
	} else {
		_ = common.HttpResponseJson(w, "OK")
	}
}

func (api *ApiServer) OnEnableSet(params *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	model, err := dao.Platform.QueryPlatformByID(params.ID)
	if err != nil {
		return nil, err
	}

	err = dao.Platform.UpdateEnable(params.ID, params.Enable)
	if err != nil {
		return nil, err
	}

	if params.Enable {
		if stack.PlatformManager.Find(model.ServerAddr) != nil {
			return nil, errors.New("device already started")
		}

		platform, err := stack.NewPlatform(&model.SIPUAOptions, common.SipStack)
		if err != nil {
			_ = dao.Platform.UpdateEnable(params.ID, false)
			return nil, err
		}

		stack.PlatformManager.Add(platform.ServerAddr, platform)
		platform.Start()
	} else if client := stack.PlatformManager.Remove(model.ServerAddr); client != nil {
		client.Stop()
	}

	return "OK", nil
}

func (api *ApiServer) OnPlatformChannelList(q *QueryCascadeChannelList, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	response := struct {
		ChannelCount int               `json:"ChannelCount"`
		ChannelList  []*CascadeChannel `json:"ChannelList"`

		ChannelRelateCount *int  `json:"ChannelRelateCount,omitempty"`
		ShareAllChannel    *bool `json:"ShareAllChannel,omitempty"`
	}{}

	id, err := strconv.Atoi(q.ID)
	if err != nil {
		return nil, err
	}

	// livegbs前端, 如果开启级联所有通道, 是不允许再只看已选择或取消绑定通道
	platform, err := dao.Platform.QueryPlatformByID(id)
	if err != nil {
		return nil, err
	}

	// 只看已选择
	if q.Related == true {
		list, total, err := dao.Platform.QueryPlatformChannelList(id)
		if err != nil {
			return nil, err
		}

		response.ChannelCount = total
		ChannelList := ChannelModels2LiveGBSChannels(q.Start+1, list, "")
		for _, channel := range ChannelList {
			response.ChannelList = append(response.ChannelList, &CascadeChannel{
				CascadeID:      q.ID,
				LiveGBSChannel: channel,
			})
		}
	} else {
		list, err := api.OnChannelList(&q.QueryDeviceChannel, w, req)
		if err != nil {
			return nil, err
		}

		result := list.(*ChannelListResult)
		response.ChannelCount = result.ChannelCount

		for _, channel := range result.ChannelList {
			var cascadeId string
			if exist, _ := dao.Platform.QueryPlatformChannelExist(id, channel.DeviceID, channel.ID); exist {
				cascadeId = q.ID
			}

			// 判断该通道是否选中
			response.ChannelList = append(response.ChannelList, &CascadeChannel{
				cascadeId, channel,
			})
		}

		response.ChannelRelateCount = new(int)
		response.ShareAllChannel = new(bool)

		// 级联设备通道总数
		if count, err := dao.Platform.QueryPlatformChannelCount(id); err != nil {
			return nil, err
		} else {
			response.ChannelRelateCount = &count
		}

		*response.ShareAllChannel = platform.ShareAll
	}

	return &response, nil
}

func (api *ApiServer) OnShareAllChannel(q *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	var err error
	if q.ShareAllChannel {
		// 删除所有已经绑定的通道, 设置级联所有通道为true
		if err = dao.Platform.DeletePlatformChannels(q.ID); err == nil {
			err = dao.Platform.SetShareAllChannel(q.ID, true)
		}
	} else {
		// 设置级联所有通道为false
		err = dao.Platform.SetShareAllChannel(q.ID, false)
	}

	if err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCustomChannelSet(q *CustomChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	if len(q.CustomID) != 20 {
		return nil, fmt.Errorf("20位国标ID")
	}

	if err := dao.Channel.UpdateCustomID(q.DeviceID, q.ChannelID, q.CustomID); err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnCatalogPush(params *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	// 使用notify发送目录列表
	model, err := dao.Platform.QueryPlatformByID(params.ID)
	if err != nil {
		return nil, err
	} else if client := stack.PlatformManager.Find(model.ServerAddr); client != nil {
		client.PushCatalog()
		return "OK", nil
	} else {
		return nil, errors.New("device not found")
	}
}
