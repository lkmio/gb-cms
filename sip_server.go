package main

import (
	"context"
	"fmt"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/util"
	"github.com/lkmio/avformat/utils"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	logger               log.Logger
	GlobalContactAddress *sip.Address
)

const (
	CmdTagStart = "<CmdType>"
	CmdTagEnd   = "</CmdType>"

	XmlNameControl  = "Control"
	XmlNameQuery    = "Query"    //主动查询消息
	XmlNameNotify   = "Notify"   //订阅产生的通知消息
	XmlNameResponse = "Response" //响应消息

	CmdDeviceInfo     = "DeviceInfo"
	CmdDeviceStatus   = "DeviceStatus"
	CmdCatalog        = "Catalog"
	CmdRecordInfo     = "RecordInfo"
	CmdMobilePosition = "MobilePosition"
	CmdKeepalive      = "Keepalive"
	CmdBroadcast      = "Broadcast"
)

func init() {
	logger = log.NewDefaultLogrusLogger().WithPrefix("Server")
}

type SipServer interface {
	SendRequestWithContext(ctx context.Context,
		request sip.Request,
		options ...gosip.RequestWithContextOption)

	SendRequest(request sip.Request) sip.ClientTransaction

	SendRequestWithTimeout(seconds int, request sip.Request, options ...gosip.RequestWithContextOption) (sip.Response, error)

	Send(msg sip.Message) error

	ListenAddr() string
}

type sipServer struct {
	sip             gosip.Server
	listenAddr      string
	xmlReflectTypes map[string]reflect.Type
	handler         EventHandler
}

type SipRequestSource struct {
	req         sip.Request
	tx          sip.ServerTransaction
	fromCascade bool
	fromJt      bool
}

func (s *sipServer) Send(msg sip.Message) error {
	return s.sip.Send(msg)
}

func setToTag(response sip.Message) {
	toHeader := response.GetHeaders("To")
	to := toHeader[0].(*sip.ToHeader)
	to.Params = sip.NewParams().Add("tag", sip.String{Str: util.RandString(10)})
}

func (s *sipServer) OnRegister(wrapper *SipRequestSource) {
	var device GBDevice
	var queryCatalog bool

	fromHeaders := wrapper.req.GetHeaders("From")
	if len(fromHeaders) == 0 {
		Sugar.Errorf("not find From header. message: %s", wrapper.req.String())
		return
	}

	_ = wrapper.req.GetHeaders("Authorization")
	fromHeader := fromHeaders[0].(*sip.FromHeader)
	expiresHeader := wrapper.req.GetHeaders("Expires")

	response := sip.NewResponseFromRequest("", wrapper.req, 200, "OK", "")
	id := fromHeader.Address.User().String()
	if len(expiresHeader) > 0 && "0" == expiresHeader[0].Value() {
		Sugar.Infof("设备注销 Device: %s", id)
		s.handler.OnUnregister(id)
	} else /*if authorizationHeader == nil*/ {
		var expires int
		expires, device, queryCatalog = s.handler.OnRegister(id, wrapper.req.Transport(), wrapper.req.Source())
		if device != nil {
			Sugar.Infof("注册成功 Device: %s addr: %s", id, wrapper.req.Source())
			expiresHeader := sip.Expires(expires)
			response.AppendHeader(&expiresHeader)
		} else {
			Sugar.Infof("注册失败 Device: %s", id)
			response = sip.NewResponseFromRequest("", wrapper.req, 401, "Unauthorized", "")
		}
	}

	SendResponse(wrapper.tx, response)

	if device != nil {
		// 查询设备信息
		device.QueryDeviceInfo()
	}

	if queryCatalog {
		device.QueryCatalog()
	}
}

// OnInvite 收到上级预览/下级设备广播请求
func (s *sipServer) OnInvite(wrapper *SipRequestSource) {
	SendResponse(wrapper.tx, sip.NewResponseFromRequest("", wrapper.req, 100, "Trying", ""))
	user := wrapper.req.Recipient().User().String()

	//if len(user) != 20 {
	//	SendResponseWithStatusCode(req, tx, http.StatusNotFound)
	//	return
	//}

	// 查找对应的设备
	var device GBDevice
	if wrapper.fromCascade {
		// 级联设备
		device = PlatformManager.Find(wrapper.req.Source())
	} else if wrapper.fromJt {
		// 部标设备
		// 1. 根据通道查找到对应的设备ID
		// 2. 根据Subject头域查找对应的设备ID
		if channels, _ := ChannelDao.QueryChannelsByChannelID(user); len(channels) > 0 {
			device = JTDeviceManager.Find(channels[0].RootID)
		}
	} else {
		if session := EarlyDialogs.Find(user); session != nil {
			// 语音广播设备
			device, _ = DeviceDao.QueryDevice(session.data.(*Sink).SinkStreamID.DeviceID())
		} else {
			// 根据Subject头域查找设备
			headers := wrapper.req.GetHeaders("Subject")
			if len(headers) > 0 {
				subject := headers[0].(*sip.GenericHeader)
				split := strings.Split(strings.Split(subject.Value(), ",")[0], ":")
				if len(split) > 1 {
					device, _ = DeviceDao.QueryDevice(split[1])
				}
			}
		}
	}

	if device == nil {
		logger.Error("处理Invite失败, 找不到设备. request: %s", wrapper.req.String())

		SendResponseWithStatusCode(wrapper.req, wrapper.tx, http.StatusNotFound)
	} else {
		response := device.OnInvite(wrapper.req, user)
		SendResponse(wrapper.tx, response)
	}
}

func (s *sipServer) OnAck(wrapper *SipRequestSource) {

}

func (s *sipServer) OnBye(wrapper *SipRequestSource) {
	response := sip.NewResponseFromRequest("", wrapper.req, 200, "OK", "")
	SendResponse(wrapper.tx, response)

	id, _ := wrapper.req.CallID()
	var deviceId string

	if stream, _ := StreamDao.DeleteStreamByCallID(id.Value()); stream != nil {
		// 下级设备挂断, 关闭流
		deviceId = stream.StreamID.DeviceID()
		stream.Close(false, true)
	} else if sink, _ := SinkDao.DeleteForwardSinkByCallID(id.Value()); sink != nil {
		sink.Close(false, true)
	}

	if wrapper.fromCascade {
		// 级联上级挂断
		if platform := PlatformManager.Find(wrapper.req.Source()); platform != nil {
			platform.OnBye(wrapper.req)
		}
	} else if wrapper.fromJt {
		// 部标设备挂断
		if jtDevice := JTDeviceManager.Find(deviceId); jtDevice != nil {
			jtDevice.OnBye(wrapper.req)
		}
	} else if device, _ := DeviceDao.QueryDevice(deviceId); device != nil {
		device.OnBye(wrapper.req)
	}
}

func (s *sipServer) OnNotify(wrapper *SipRequestSource) {
	response := sip.NewResponseFromRequest("", wrapper.req, 200, "OK", "")
	SendResponse(wrapper.tx, response)

	mobilePosition := MobilePositionNotify{}
	if err := DecodeXML([]byte(wrapper.req.Body()), &mobilePosition); err != nil {
		Sugar.Errorf("解析位置通知失败 err: %s request: %s", err.Error(), wrapper.req.String())
		return
	}

	s.handler.OnNotifyPosition(&mobilePosition)
}

func (s *sipServer) OnMessage(wrapper *SipRequestSource) {
	var ok bool
	defer func() {
		var response sip.Response
		if ok {
			response = CreateResponseWithStatusCode(wrapper.req, http.StatusOK)
		} else {
			response = CreateResponseWithStatusCode(wrapper.req, http.StatusForbidden)
		}

		SendResponse(wrapper.tx, response)
	}()

	body := wrapper.req.Body()
	xmlName := GetRootElementName(body)
	cmd := GetCmdType(body)
	src, ok := s.xmlReflectTypes[xmlName+"."+cmd]
	if !ok {
		Sugar.Errorf("处理XML消息失败, 找不到结构体. request: %s", wrapper.req.String())
		return
	}

	message := reflect.New(src).Interface()
	if err := DecodeXML([]byte(body), message); err != nil {
		Sugar.Errorf("解析XML消息失败 err: %s request: %s", err.Error(), body)
		return
	}

	// 查找设备
	deviceId := message.(BaseMessageGetter).GetDeviceID()
	if CmdBroadcast == cmd {
		// 广播消息
		from, _ := wrapper.req.From()
		deviceId = from.Address.User().String()
	}

	switch xmlName {
	case XmlNameControl:
		break
	case XmlNameQuery:
		// 被上级查询
		var device GBClient
		if wrapper.fromCascade {
			device = PlatformManager.Find(wrapper.req.Source())
		} else if wrapper.fromJt {
			device = JTDeviceManager.Find(deviceId)
		}

		if ok = device != nil; !ok {
			Sugar.Errorf("处理上级请求消息失败, 找不到设备 addr: %s request: %s", wrapper.req.Source(), wrapper.req.String())
			return
		}

		if CmdDeviceInfo == cmd {
			device.OnQueryDeviceInfo(message.(*BaseMessage).SN)
		} else if CmdCatalog == cmd {
			var channels []*Channel

			// 查询出所有通道
			if wrapper.fromCascade {
				result, err := PlatformDao.QueryPlatformChannels(device.GetDomain())
				if err != nil {
					Sugar.Errorf("查询设备通道列表失败 err: %s device: %s", err.Error(), device.GetID())
				}

				channels = result
			} else if wrapper.fromJt {
				channels, _ = ChannelDao.QueryChannelsByRootID(device.GetID())
			} else {
				// 从模拟多个国标客户端中查找
				channels = DeviceChannelsManager.FindChannels(device.GetID())
			}

			device.OnQueryCatalog(message.(*BaseMessage).SN, channels)
		}

		break
	case XmlNameNotify:
		if CmdKeepalive == cmd {
			// 下级设备心跳通知
			ok = s.handler.OnKeepAlive(deviceId, wrapper.req.Source())
		}

		break

	case XmlNameResponse:
		// 查询下级的应答
		if CmdCatalog == cmd {
			s.handler.OnCatalog(deviceId, message.(*CatalogResponse))
		} else if CmdRecordInfo == cmd {
			s.handler.OnRecord(deviceId, message.(*QueryRecordInfoResponse))
		} else if CmdDeviceInfo == cmd {
			s.handler.OnDeviceInfo(deviceId, message.(*DeviceInfoResponse))
		}

		break
	}
}

func CreateResponseWithStatusCode(request sip.Request, code int) sip.Response {
	return sip.NewResponseFromRequest("", request, sip.StatusCode(code), StatusCode2Reason(code), "")
}

func SendResponseWithStatusCode(request sip.Request, tx sip.ServerTransaction, code int) {
	SendResponse(tx, CreateResponseWithStatusCode(request, code))
}

func SendResponse(tx sip.ServerTransaction, response sip.Response) bool {
	sendError := tx.Respond(response)

	if sendError != nil {
		Sugar.Errorf("发送响应消息失败, error: %s response: %s", sendError.Error(), response.String())
	}

	return sendError == nil
}

func (s *sipServer) SendRequestWithContext(ctx context.Context, request sip.Request, options ...gosip.RequestWithContextOption) {
	s.sip.RequestWithContext(ctx, request, options...)
}

func (s *sipServer) SendRequestWithTimeout(seconds int, request sip.Request, options ...gosip.RequestWithContextOption) (sip.Response, error) {
	reqCtx, _ := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	return s.sip.RequestWithContext(reqCtx, request, options...)
}

func (s *sipServer) SendRequest(request sip.Request) sip.ClientTransaction {
	transaction, err := s.sip.Request(request)
	if err != nil {
		panic(err)
	}

	return transaction
}

func (s *sipServer) ListenAddr() string {
	return s.listenAddr
}

// 过滤SIP消息、超找消息来源
func filterRequest(f func(wrapper *SipRequestSource)) gosip.RequestHandler {
	return func(req sip.Request, tx sip.ServerTransaction) {

		source := req.Source()
		// 是否是级联上级下发的请求
		platform := PlatformManager.Find(source)
		// 是否是部标设备上级下发的请求
		var fromJt bool
		if platform == nil {
			fromJt = JTDeviceManager.ExistClientByServerAddr(req.Source())
		}
		switch req.Method() {
		case sip.SUBSCRIBE, sip.INFO:
			if platform == nil || fromJt {
				// SUBSCRIBE/INFO只能本级域向下级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
				Sugar.Errorf("处理%s请求失败, %s消息只能上级发起. request: %s", req.Method(), req.Method(), req.String())
				return
			}
			break
		case sip.NOTIFY, sip.REGISTER:
			if platform != nil || fromJt {
				// NOTIFY和REGISTER只能下级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
				Sugar.Errorf("处理%s请求失败, %s消息只能下级发起. request: %s", req.Method(), req.Method(), req.String())
				return
			}
			break
		}

		f(&SipRequestSource{
			req,
			tx,
			platform != nil,
			fromJt,
		})
	}
}

func StartSipServer(id, listenIP, publicIP string, listenPort int) (SipServer, error) {
	ua := gosip.NewServer(gosip.ServerConfig{
		Host:      publicIP,
		UserAgent: "github/lkmio",
	}, nil, nil, logger)

	addr := net.JoinHostPort(listenIP, strconv.Itoa(listenPort))
	if err := ua.Listen("udp", addr); err != nil {
		return nil, err
	} else if err := ua.Listen("tcp", addr); err != nil {
		return nil, err
	}

	server := &sipServer{sip: ua, xmlReflectTypes: map[string]reflect.Type{
		fmt.Sprintf("%s.%s", XmlNameQuery, CmdCatalog):         reflect.TypeOf(BaseMessage{}),
		fmt.Sprintf("%s.%s", XmlNameQuery, CmdDeviceInfo):      reflect.TypeOf(BaseMessage{}),
		fmt.Sprintf("%s.%s", XmlNameQuery, CmdDeviceStatus):    reflect.TypeOf(BaseMessage{}),
		fmt.Sprintf("%s.%s", XmlNameResponse, CmdCatalog):      reflect.TypeOf(CatalogResponse{}),
		fmt.Sprintf("%s.%s", XmlNameResponse, CmdDeviceInfo):   reflect.TypeOf(DeviceInfoResponse{}),
		fmt.Sprintf("%s.%s", XmlNameResponse, CmdDeviceStatus): reflect.TypeOf(DeviceStatusResponse{}),
		fmt.Sprintf("%s.%s", XmlNameResponse, CmdRecordInfo):   reflect.TypeOf(QueryRecordInfoResponse{}),
		fmt.Sprintf("%s.%s", XmlNameNotify, CmdKeepalive):      reflect.TypeOf(BaseMessage{}),
		fmt.Sprintf("%s.%s", XmlNameNotify, CmdMobilePosition): reflect.TypeOf(BaseMessage{}),
		fmt.Sprintf("%s.%s", XmlNameResponse, CmdBroadcast):    reflect.TypeOf(BaseMessage{}),
	}}

	utils.Assert(ua.OnRequest(sip.REGISTER, filterRequest(server.OnRegister)) == nil)
	utils.Assert(ua.OnRequest(sip.INVITE, filterRequest(server.OnInvite)) == nil)
	utils.Assert(ua.OnRequest(sip.BYE, filterRequest(server.OnBye)) == nil)
	utils.Assert(ua.OnRequest(sip.ACK, filterRequest(server.OnAck)) == nil)
	utils.Assert(ua.OnRequest(sip.NOTIFY, filterRequest(server.OnNotify)) == nil)
	utils.Assert(ua.OnRequest(sip.MESSAGE, filterRequest(server.OnMessage)) == nil)

	utils.Assert(ua.OnRequest(sip.INFO, filterRequest(func(wrapper *SipRequestSource) {
	})) == nil)
	utils.Assert(ua.OnRequest(sip.CANCEL, filterRequest(func(wrapper *SipRequestSource) {
	})) == nil)
	utils.Assert(ua.OnRequest(sip.SUBSCRIBE, filterRequest(func(wrapper *SipRequestSource) {
	})) == nil)

	server.listenAddr = addr
	port := sip.Port(listenPort)
	GlobalContactAddress = &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: id},
			FHost: publicIP,
			FPort: &port,
		},
	}

	return server, nil
}
