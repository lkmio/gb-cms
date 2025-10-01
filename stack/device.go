package stack

import (
	"context"
	"errors"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	CatalogFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>Catalog</CmdType>\r\n" +
		"<SN>" +
		"%d" +
		"</SN>\r\n" +
		"<DeviceID>" +
		"%s" +
		"</DeviceID>\r\n" +
		"</Query>\r\n"

	DeviceInfoFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>DeviceInfo</CmdType>\r\n" +
		"<SN>" +
		"%d" +
		"</SN>\r\n" +
		"<DeviceID>" +
		"%s" +
		"</DeviceID>\r\n" +
		"</Query>\r\n"
)

var (
	XmlMessageType sip.ContentType = "Application/MANSCDP+xml"

	SDPMessageType sip.ContentType = "application/sdp"

	RTSPMessageType sip.ContentType = "application/RTSP"
)

type GBDevice interface {
	GetID() string

	// QueryDeviceInfo 发送查询设备信息命令
	QueryDeviceInfo()

	// QueryCatalog 发送查询目录命令
	QueryCatalog(timeout int) ([]*dao.ChannelModel, error)

	// QueryRecord 发送查询录像命令
	QueryRecord(channelId, startTime, endTime string, sn int, type_ string) error

	//Invite(channel string, setup string)

	// OnInvite 语音广播
	OnInvite(request sip.Request, user string) sip.Response

	// OnBye 设备侧主动挂断
	OnBye(request sip.Request)

	//
	//OnNotifyCatalog()
	//
	//OnNotifyAlarm()

	SubscribePosition(channelId string) error

	//SubscribeCatalog()
	//
	//SubscribeAlarm()

	Broadcast(sourceId, channelId string) sip.ClientTransaction

	// UpdateChannel 订阅目录，通道发生改变
	// 附录P.4.2.2
	// @Params event ON-上线/OFF-离线/VLOST-视频丢失/DEFECT-故障/ADD-增加/DEL-删除/UPDATE-更新
	UpdateChannel(id string, event string)

	Close()
}

type CatalogProgress struct {
	TotalSize int
	RecvSize  int
}

type Device struct {
	*dao.DeviceModel
}

func (d *Device) BuildMessageRequest(to, body string) sip.Request {
	request, err := BuildMessageRequest(common.Config.SipID, net.JoinHostPort(GlobalContactAddress.Uri.Host(), GlobalContactAddress.Uri.Port().String()), to, net.JoinHostPort(d.RemoteIP, strconv.Itoa(d.RemotePort)), d.Transport, body)
	if err != nil {
		panic(err)
	}

	return request
}

func (d *Device) QueryDeviceInfo() {
	body := fmt.Sprintf(DeviceInfoFormat, GetSN(), d.DeviceID)
	request := d.BuildMessageRequest(d.DeviceID, body)
	common.SipStack.SendRequest(request)
}

func (d *Device) QueryCatalog(timeoutSeconds int) ([]*dao.ChannelModel, error) {
	catalogProgress := &CatalogProgress{}

	var timeoutCtx context.Context
	var timeoutCancelFunc context.CancelFunc
	if timeoutSeconds > 0 {
		timeoutCtx, timeoutCancelFunc = context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	}

	var err error
	var result []*dao.ChannelModel
	query := func() {
		defer func() {
			if timeoutCancelFunc != nil {
				timeoutCancelFunc()
			}
		}()

		// 下发查询指令
		finish := make(chan byte, 1)
		sn := GetSN()
		body := fmt.Sprintf(CatalogFormat, sn, d.DeviceID)
		request := d.BuildMessageRequest(d.DeviceID, body)
		tx := common.SipStack.SendRequest(request)
		// 异步等待响应
		go func() {
			response := <-tx.Responses()
			if response != nil && response.StatusCode() != http.StatusOK {
				err = fmt.Errorf("query catalog res[%d] %s", response.StatusCode(), StatusCode2Reason(int(response.StatusCode())))
				finish <- 1
				return
			}
		}()

		// 处理目录消息
		lastTime := time.Now()
		var list []*CatalogResponse
		SNManager.AddEvent(sn, func(response interface{}) {
			lastTime = time.Now()
			catalog := response.(*CatalogResponse)
			catalogProgress.TotalSize = catalog.SumNum
			catalogProgress.RecvSize += catalog.DeviceList.Num

			list = append(list, catalog)
			if catalogProgress.RecvSize >= catalogProgress.TotalSize {
				finish <- 1
			}
		})

		// 定时检测是否超时或完成
		timeout := 10 * time.Second
		ticker := time.NewTicker(timeout)
		for {
			var end bool
			select {
			case <-ticker.C:
				if time.Since(lastTime) > timeout {
					// 超时, 则直接返回
					err = fmt.Errorf("query catalog timeout[%ds]", int(timeout.Seconds()))
					ticker.Stop()
					end = true
					break
				}
			case <-finish:
				ticker.Stop()
				end = true
				break
			}

			if end {
				break
			}
		}

		if err != nil {
			return
		}

		// 如果查询不完整, 并且数据库中通道列表不为空, 则丢弃本次查询的数据, 否则依旧入库
		var oldChannelCount int
		oldChannelCount, err = dao.Channel.QueryChanelCount(d.DeviceID, true)
		if err != nil {
			log.Sugar.Errorf("query channel count failed, device: %s, err: %s", d.DeviceID, err.Error())
			return
		} else if len(list) < 1 || (oldChannelCount > 0 && catalogProgress.RecvSize < catalogProgress.TotalSize) {
			log.Sugar.Errorf("query catalog failed, device: %s, count: %d, recvSize: %d, totalSize: %d", d.DeviceID, oldChannelCount, catalogProgress.RecvSize, catalogProgress.TotalSize)
			return
		}

		// 删除旧的通道列表
		if oldChannelCount > 0 {
			err = dao.Channel.DeleteChannels(d.DeviceID)
			if err != nil {
				log.Sugar.Errorf("delete channels failed, device: %s, err: %s", d.DeviceID, err.Error())
				return
			}
		}

		// 批量保存通道
		result, err = d.SaveChannels(list)
		// 更新查询目录的时间
		_ = dao.Device.UpdateRefreshCatalogTime(d.DeviceID, time.Now())
	}

	if !UniqueTaskManager.Commit(GenerateCatalogTaskID(d.DeviceID), query, catalogProgress) {
		return nil, errors.New("device busy")
	}

	// web接口的查询超时
	if timeoutCtx != nil {
		select {
		case <-timeoutCtx.Done():
			if err == nil && catalogProgress.RecvSize < catalogProgress.TotalSize {
				err = fmt.Errorf("wait for catalog[%d/%d] timeout[%ds]", catalogProgress.RecvSize, catalogProgress.TotalSize, timeoutSeconds)
			}
			break
		}
	}

	return result, err
}

func IsDir(typeCode int) bool {
	return typeCode < 131 || typeCode > 199
}

func (d *Device) SaveChannels(list []*CatalogResponse) ([]*dao.ChannelModel, error) {
	var channels []*dao.ChannelModel
	// 目录
	dirs := make(map[string]*dao.ChannelModel)
	for _, response := range list {
		for _, channel := range response.DeviceList.Devices {
			// 状态转为大写
			channel.Status = common.OnlineStatus(strings.ToUpper(channel.Status.String()))

			// 默认在线
			if common.OFF != channel.Status {
				channel.Status = common.ON
			}

			// 下级设备的系统ID, 更新DeviceInfo
			if channel.DeviceID == d.DeviceID && dao.Device.ExistDevice(d.DeviceID) {
				_ = dao.Device.UpdateDeviceInfo(d.DeviceID, &dao.DeviceModel{
					Manufacturer: channel.Manufacturer,
					Model:        channel.Model,
					Name:         channel.Name,
				})
			}

			typeCode := GetTypeCode(channel.DeviceID)
			if typeCode == "" {
				log.Sugar.Errorf("保存通道时, 获取设备类型失败 device: %s", channel.DeviceID)
			}

			// 通道所属组, ParentID优先, 其次BusinessGroupID
			var groupId string
			if channel.ParentID != "" {
				layers := strings.Split(channel.ParentID, "/")
				groupId = layers[len(layers)-1]
			} else if channel.BusinessGroupID != "" {
				groupId = channel.BusinessGroupID
			}

			code, _ := strconv.Atoi(typeCode)
			channel.RootID = d.DeviceID
			channel.TypeCode = code
			channel.GroupID = groupId
			channels = append(channels, channel)

			dirs[channel.RootID+"/"+channel.DeviceID] = channel
		}
	}

	// 父通道不是目录, 归属到最近的目录或设备, 所有外围设备同级
	for _, channel := range channels {
		for {
			parentChannel, ok := dirs[channel.RootID+"/"+channel.GroupID]
			if !ok {
				break
			} else if !IsDir(parentChannel.TypeCode) {
				channel.GroupID = parentChannel.GroupID
			} else {
				break
			}
		}
	}

	// 统计目录的子通道数量
	for _, channel := range channels {
		if parentChannel, ok := dirs[channel.RootID+"/"+channel.GroupID]; ok && IsDir(parentChannel.TypeCode) {
			parentChannel.IsDir = true
			parentChannel.SubCount++
		}
	}

	err := dao.Channel.SaveChannels(channels)
	if err != nil {
		log.Sugar.Errorf("save channels failed, device: %s, err: %s", d.DeviceID, err.Error())
		return nil, err
	}

	return channels, nil
}

func (d *Device) QueryRecord(channelId, startTime, endTime string, sn int, type_ string) error {
	body := fmt.Sprintf(QueryRecordFormat, sn, channelId, startTime, endTime, type_)
	request := d.BuildMessageRequest(channelId, body)
	common.SipStack.SendRequest(request)
	return nil
}

func (d *Device) OnBye(request sip.Request) {

}

func (d *Device) SubscribePosition(channelId string) error {
	if channelId == "" {
		channelId = d.DeviceID
	}

	//暂时不考虑级联
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, common.Config.SipID, common.Config.SipContactAddr, channelId)
	body := fmt.Sprintf(MobilePositionMessageFormat, GetSN(), channelId, common.Config.MobilePositionInterval)

	expiresHeader := sip.Expires(common.Config.MobilePositionExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	event := Event(EventPresence)
	request.AppendHeader(&event)
	response, err := common.SipStack.SendRequestWithTimeout(5, request)
	if err != nil {
		return err
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("err code %d", response.StatusCode())
	}

	return nil
}

func (d *Device) Broadcast(sourceId, channelId string) sip.ClientTransaction {
	body := fmt.Sprintf(BroadcastFormat, GetSN(), sourceId, channelId)
	request := d.BuildMessageRequest(channelId, body)
	return common.SipStack.SendRequest(request)
}

func (d *Device) UpdateChannel(id string, event string) {

}

func (d *Device) BuildCatalogRequest() (sip.Request, error) {
	body := fmt.Sprintf(CatalogFormat, GetSN(), d.DeviceID)
	request := d.BuildMessageRequest(d.DeviceID, body)
	return request, nil
}

func (d *Device) NewSIPRequestBuilderWithTransport() *sip.RequestBuilder {
	builder := sip.NewRequestBuilder()
	hop := sip.ViaHop{
		Transport: d.Transport,
	}

	builder.AddVia(&hop)
	return builder
}

func (d *Device) NewRequestBuilder(method sip.RequestMethod, fromUser, realm, toUser string) *sip.RequestBuilder {
	builder := d.NewSIPRequestBuilderWithTransport()
	builder.SetMethod(method)

	sipPort := sip.Port(d.RemotePort)

	requestUri := &sip.SipUri{
		FUser: sip.String{Str: toUser},
		FHost: d.RemoteIP,
		FPort: &sipPort,
	}

	builder.SetRecipient(requestUri)

	fromAddress := &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: fromUser},
			FHost: realm,
		},
	}

	fromAddress.Params = sip.NewParams().Add("tag", sip.String{Str: GenerateTag()})
	builder.SetFrom(fromAddress)
	builder.SetTo(&sip.Address{
		Uri: requestUri,
	})

	return builder
}

func (d *Device) BuildInviteRequest(sessionName, channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	builder := d.NewRequestBuilder(sip.INVITE, common.Config.SipID, common.Config.SipContactAddr, channelId)
	sdp := BuildSDP("video", common.Config.SipID, sessionName, ip, port, startTime, stopTime, setup, speed, ssrc, "96 PS/90000")
	builder.SetContentType(&SDPMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(sdp)
	request, err := builder.Build()
	if err != nil {
		return nil, err
	}

	var subjectHeader = Subject(channelId + ":" + d.DeviceID + "," + common.Config.SipID + ":" + ssrc)
	request.AppendHeader(subjectHeader)

	return request, err
}

func (d *Device) BuildLiveRequest(channelId, ip string, port uint16, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Play", channelId, ip, port, "0", "0", setup, 0, ssrc)
}

func (d *Device) BuildPlaybackRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Playback", channelId, ip, port, startTime, stopTime, setup, 0, ssrc)
}

func (d *Device) BuildDownloadRequest(channelId, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string) (sip.Request, error) {
	return d.BuildInviteRequest("Download", channelId, ip, port, startTime, stopTime, setup, speed, ssrc)
}

func (d *Device) Close() {
	// 更新在数据库中的状态
	d.Status = common.OFF
	_ = dao.Device.UpdateDeviceStatus(d.DeviceID, common.OFF)
}

// CreateDialogRequestFromAnswer 根据invite的应答创建Dialog请求
// 应答的to头域需携带tag
func CreateDialogRequestFromAnswer(message sip.Response, uas bool, remoteAddr string) sip.Request {
	from, _ := message.From()
	to, _ := message.To()
	id, _ := message.CallID()

	requestLine := &sip.SipUri{}
	host, port, _ := net.SplitHostPort(remoteAddr)
	portInt, _ := strconv.Atoi(port)
	sipPort := sip.Port(portInt)
	requestLine.SetHost(host)
	requestLine.SetPort(&sipPort)

	seq, _ := message.CSeq()

	builder := NewSIPRequestBuilderWithTransport(message.Transport())
	if uas {
		requestLine.SetUser(from.Address.User())
		builder.SetFrom(sip.NewAddressFromToHeader(to))
		builder.SetTo(sip.NewAddressFromFromHeader(from))
	} else {
		requestLine.SetUser(to.Address.User())
		builder.SetFrom(sip.NewAddressFromFromHeader(from))
		builder.SetTo(sip.NewAddressFromToHeader(to))
	}

	builder.SetCallID(id)
	builder.SetMethod(sip.BYE)
	builder.SetRecipient(requestLine)
	builder.SetSeqNo(uint(seq.SeqNo + 1))
	request, err := builder.Build()
	if err != nil {
		panic(err)
	}

	return request
}

func (d *Device) CreateDialogRequestFromAnswer(message sip.Response, uas bool) sip.Request {
	return CreateDialogRequestFromAnswer(message, uas, net.JoinHostPort(d.RemoteIP, strconv.Itoa(d.RemotePort)))
}
