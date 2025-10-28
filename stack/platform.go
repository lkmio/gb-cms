package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	UATypeGB = iota + 1
	UATypeJT
)

type Platform struct {
	*gbClient
	registerTimer *time.Timer // 在发起注册时, 启动定时器, 到期后, 如果未上线, 释放资源. 防止重启后, 上级也重启, 资源长时间未释放.
}

// OnBye 被上级挂断
func (g *Platform) OnBye(request sip.Request) {
	id, _ := request.CallID()
	g.CloseStream(id.Value(), false, true)
}

// 追加本地域
func (g *Platform) appendLocalDomain(channels []*dao.ChannelModel) []*dao.ChannelModel {
	return append(channels, &dao.ChannelModel{
		DeviceID:     g.ServerID,
		Setup:        common.SetupTypePassive,
		Name:         DefaultDomainName,
		Manufacturer: DefaultManufacturer,
		Model:        DefaultModel,
		Owner:        "Owner",
		Address:      "Address",
		ParentID:     "0",
		Secrecy:      "0",
		RegisterWay:  "1",
	})
}

func (g *Platform) OnQueryCatalog(sn int, channels []*dao.ChannelModel) {
	if len(channels) < 1 {
		return
	}

	g.gbClient.OnQueryCatalog(sn, g.appendLocalDomain(channels))
}

// CloseStream 关闭级联会话
func (g *Platform) CloseStream(callId string, bye, ms bool) {
	sink, _ := dao.Sink.DeleteSinkByCallID(callId)
	if sink != nil {
		(&Sink{sink}).Close(bye, ms)
	}
}

// CloseStreams 关闭所有级联会话
func (g *Platform) CloseStreams(bye, ms bool) {
	sinks, _ := dao.Sink.DeleteSinksByServerAddr(g.ServerAddr)
	for _, sink := range sinks {
		(&Sink{sink}).Close(bye, ms)
	}
}

// OnInvite 被上级呼叫
func (g *Platform) OnInvite(request sip.Request, user string) sip.Response {
	log.Sugar.Infof("收到上级Invite请求 platform: %s channel: %s sdp: %s", g.ServerID, user, request.Body())

	source := request.Source()
	platform := PlatformManager.Find(source)
	utils.Assert(platform != nil)

	deviceId, channel, err := dao.Platform.QueryPlatformChannel(g.ServerAddr, user)
	if err != nil {
		log.Sugar.Errorf("处理上级Invite失败, 查询数据库失败 err: %s platform: %s channel: %s", err.Error(), g.ServerID, user)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	// 查找通道对应的设备
	device, _ := dao.Device.QueryDevice(deviceId)
	if device == nil {
		log.Sugar.Errorf("处理上级Invite失败, 设备不存在 device: %s channel: %s", device, user)
		return CreateResponseWithStatusCode(request, http.StatusNotFound)
	}

	// 解析sdp
	gbSdp, err := ParseGBSDP(request.Body())
	if err != nil {
		log.Sugar.Errorf("处理上级Invite失败,err: %s sdp: %s", err.Error(), request.Body())
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	var inviteType common.InviteType
	inviteType.SessionName2Type(strings.ToLower(gbSdp.SDP.Session))
	streamId := common.GenerateStreamID(inviteType, channel.RootID, channel.DeviceID, gbSdp.StartTime, gbSdp.StopTime)

	sink := &dao.SinkModel{
		StreamID:   streamId,
		ServerAddr: g.ServerAddr,
		Protocol:   TransStreamGBCascaded}

	// 添加转发sink到流媒体服务器
	response, err := AddForwardSink(TransStreamGBCascaded, request, user, &Sink{sink}, streamId, gbSdp, inviteType, "96 PS/90000")
	if err != nil {
		log.Sugar.Errorf("处理上级Invite失败 err: %s stream: %s", err.Error(), streamId)
		response = CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	return response
}

func (g *Platform) Start() {
	log.Sugar.Infof("启动级联设备, deivce: %s transport: %s addr: %s", g.Username, g.sipUA.Transport, g.sipUA.ServerAddr)
	g.registerTimer = time.AfterFunc(120*time.Second, func() {
		if !g.Online() {
			g.release()
		}
	})

	g.sipUA.Start()
	g.sipUA.SetOnRegisterHandler(g.OnlineCB, g.OfflineCB)
}

func (g *Platform) Stop() {
	g.sipUA.Stop()
	g.sipUA.SetOnRegisterHandler(nil, nil)

	g.release()
}

func (g *Platform) OnlineCB() {
	if g.registerTimer != nil {
		g.registerTimer.Stop()
		g.registerTimer = nil
	}

	log.Sugar.Infof("级联设备上线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := dao.Platform.UpdateOnlineStatus(common.ON, g.ServerAddr); err != nil {
		log.Sugar.Infof("更新级联设备状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}
}

func (g *Platform) OfflineCB() {
	log.Sugar.Infof("级联设备离线 device: %s server addr: %s", g.Username, g.ServerAddr)

	if err := dao.Platform.UpdateOnlineStatus(common.OFF, g.ServerAddr); err != nil {
		log.Sugar.Infof("更新级联设备状态失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
	}

	g.release()
}

func (g *Platform) release() {
	// 取消注册定时器
	if g.registerTimer != nil {
		g.registerTimer.Stop()
		g.registerTimer = nil
	}

	// 释放所有级联会话
	g.CloseStreams(true, true)

	// 删除订阅会话
	_ = dao.Dialog.DeleteDialogs(g.ServerAddr)
}

func (g *Platform) OnSubscribeCatalog(request sip.Request, expires int) (sip.Response, error) {
	id := request.Source()
	return CreateOrDeleteSubscribeDialog(id, request, expires, dao.SipDialogTypeSubscribeCatalog)
}

func (g *Platform) OnSubscribeAlarm(request sip.Request, expires int) (sip.Response, error) {
	id := request.Source()
	return CreateOrDeleteSubscribeDialog(id, request, expires, dao.SipDialogTypeSubscribeAlarm)
}

func (g *Platform) CreateRequestByDialogType(t int, method sip.RequestMethod) (sip.Request, error) {
	model, err := dao.Dialog.QueryDialogsByType(g.ServerAddr, t)
	if err != nil {
		return nil, err
	} else if len(model) < 1 || model[0].Dialog == nil {
		return nil, fmt.Errorf("dialog type %d not found", t)
	}

	host, p, _ := net.SplitHostPort(g.ServerAddr)
	remotePort, _ := strconv.Atoi(p)

	if seq, b := model[0].Dialog.Request.CSeq(); model[0].CSeqNumber > 0 && b {
		seq.SeqNo = model[0].CSeqNumber
	}

	request := CreateRequestFromDialog(model[0].Dialog.Request, method, host, remotePort)

	// 添加头域
	expiresSeconds := model[0].RefreshTime.Sub(time.Now()).Seconds()
	subscriptionState := SubscriptionState(fmt.Sprintf("active;expires=%.0f;retry-after=0", expiresSeconds))
	event := Event("presence")
	if dao.SipDialogTypeSubscribeCatalog == t {
		event = "catalog"
	}

	common.SetHeaderIfNotExist(request, &event)
	common.SetHeader(request, &subscriptionState)
	common.SetHeader(request, &XmlMessageType)
	common.SetHeader(request, GlobalContactAddress.AsContactHeader())
	if seq, b := request.CSeq(); b {
		_ = dao.Dialog.UpdateCSeqNumber(model[0].CallID, seq.SeqNo)
	}
	return request, nil
}

func (g *Platform) PushCatalog() {
	channels, err := dao.Platform.QueryPlatformChannels(g.ServerAddr)
	if err != nil {
		log.Sugar.Errorf("查询级联设备通道失败 err: %s server addr: %s", err.Error(), g.ServerAddr)
		return
	} else if len(channels) < 1 {
		return
	}

	for _, channel := range channels {
		channel.Event = "ADD"
	}

	// 因为没有dialog, 可能有的协议栈发送不过去
	g.NotifyCatalog(GetSN(), g.appendLocalDomain(channels), func() sip.Request {
		request, err := BuildRequest(sip.NOTIFY, g.sipUA.Username, g.sipUA.ListenAddr, g.sipUA.ServerID, g.sipUA.ServerAddr, g.sipUA.Transport, nil, "")
		if err != nil {
			panic(err)
		}

		event := Event("catalog")
		subscriptionState := SubscriptionState("active")
		common.SetHeader(request, &event)
		common.SetHeader(request, &subscriptionState)
		return request
	})
}

func CreateOrDeleteSubscribeDialog(id string, request sip.Request, expires int, t int) (sip.Response, error) {
	response := sip.NewResponseFromRequest("", request, 200, "OK", "")
	common.SetHeader(response, GlobalContactAddress.AsContactHeader())

	if expires < 1 {
		// 取消订阅, 删除会话
		_, _ = dao.Dialog.DeleteDialogsByType(id, t)
	} else {
		// 设置to tag
		toHeader, _ := response.To()

		var tag string
		if toHeader.Params != nil {
			get, b := toHeader.Params.Get("tag")
			if b {
				tag = get.String()
			}
		}
		if "" == tag {
			common.SetToTag(response)
		}

		// 首次或刷新订阅, 保存或更新会话
		dialog := CreateDialogRequestFromAnswer(response, true, request.Source())
		callid, _ := dialog.CallID()

		refreshTime := time.Now().Add(time.Duration(expires) * time.Second)

		// 已经存在会话, 更新刷新时间
		oldDialog, err := dao.Dialog.QueryDialogByCallID(callid.Value())
		if err == nil && oldDialog.ID > 0 {
			oldDialog.RefreshTime = time.Now().Add(time.Duration(expires) * time.Second)
			err = dao.Dialog.UpdateRefreshTime(callid.Value(), refreshTime)
			return nil, err
		}

		// 创建新会话
		// 删除之前旧的
		_, _ = dao.Dialog.DeleteDialogsByType(id, t)

		// 保存会话
		seq, _ := dialog.CSeq()
		err = dao.Dialog.Save(&dao.SipDialogModel{
			DeviceID:    id,
			CallID:      callid.Value(),
			Dialog:      &common.RequestWrapper{Request: dialog},
			Type:        t,
			RefreshTime: refreshTime,
			CSeqNumber:  seq.SeqNo,
		})

		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func NewPlatform(options *common.SIPUAOptions, ua common.SipServer) (*Platform, error) {
	if len(options.ServerID) != 20 {
		return nil, fmt.Errorf("ServerID must be exactly 20 characters long")
	}

	if _, err := netip.ParseAddrPort(options.ServerAddr); err != nil {
		return nil, err
	}

	// 防止在重启sip阶段, 出现创建级联设备的情况
	sipLock.RLock()
	defer sipLock.RUnlock()

	client := NewGBClient(options, ua)
	return &Platform{gbClient: client.(*gbClient)}, nil
}
