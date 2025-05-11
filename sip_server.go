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

func (s *sipServer) Send(msg sip.Message) error {
	return s.sip.Send(msg)
}

func setToTag(response sip.Message) {
	toHeader := response.GetHeaders("To")
	to := toHeader[0].(*sip.ToHeader)
	to.Params = sip.NewParams().Add("tag", sip.String{Str: util.RandString(10)})
}

func (s *sipServer) OnRegister(req sip.Request, tx sip.ServerTransaction, parent bool) {
	var device GBDevice
	var queryCatalog bool

	fromHeaders := req.GetHeaders("From")
	if len(fromHeaders) == 0 {
		Sugar.Errorf("not find From header. message: %s", req.String())
		return
	}

	_ = req.GetHeaders("Authorization")
	fromHeader := fromHeaders[0].(*sip.FromHeader)
	expiresHeader := req.GetHeaders("Expires")

	response := sip.NewResponseFromRequest("", req, 200, "OK", "")
	id := fromHeader.Address.User().String()
	if len(expiresHeader) > 0 && "0" == expiresHeader[0].Value() {
		Sugar.Infof("设备注销 Device: %s", id)
		s.handler.OnUnregister(id)
	} else /*if authorizationHeader == nil*/ {
		var expires int
		expires, device, queryCatalog = s.handler.OnRegister(id, req.Transport(), req.Source())
		if device != nil {
			Sugar.Infof("注册成功 Device: %s addr: %s", id, req.Source())
			expiresHeader := sip.Expires(expires)
			response.AppendHeader(&expiresHeader)
		} else {
			Sugar.Infof("注册失败 Device: %s", id)
			response = sip.NewResponseFromRequest("", req, 401, "Unauthorized", "")
		}
	}

	SendResponse(tx, response)

	if device != nil {
		// 查询设备信息
		device.QueryDeviceInfo()
	}

	if queryCatalog {
		device.QueryCatalog()
	}
}

// OnInvite 收到上级预览/下级设备广播请求
func (s *sipServer) OnInvite(req sip.Request, tx sip.ServerTransaction, parent bool) {
	SendResponse(tx, sip.NewResponseFromRequest("", req, 100, "Trying", ""))
	user := req.Recipient().User().String()

	//if len(user) != 20 {
	//	SendResponseWithStatusCode(req, tx, http.StatusNotFound)
	//	return
	//}

	// 查找对应的设备
	var device GBDevice
	if parent {
		// 级联设备
		device = PlatformManager.Find(req.Source())
	} else if session := BroadcastDialogs.Find(user); session != nil {
		// 语音广播设备
		device = DeviceManager.Find(session.SinkStream.DeviceID())
	} else {
		// 根据Subject头域查找设备
		headers := req.GetHeaders("Subject")
		if len(headers) > 0 {
			subject := headers[0].(*sip.GenericHeader)
			split := strings.Split(strings.Split(subject.Value(), ",")[0], ":")
			if len(split) > 1 {
				device = DeviceManager.Find(split[1])
			}
		}
	}

	if device == nil {
		logger.Error("处理Invite失败, 找不到设备. request: %s", req.String())

		SendResponseWithStatusCode(req, tx, http.StatusNotFound)
	} else {
		response := device.OnInvite(req, user)
		SendResponse(tx, response)
	}
}

func (s *sipServer) OnAck(req sip.Request, tx sip.ServerTransaction, parent bool) {

}

func (s *sipServer) OnBye(req sip.Request, tx sip.ServerTransaction, parent bool) {
	response := sip.NewResponseFromRequest("", req, 200, "OK", "")
	SendResponse(tx, response)

	id, _ := req.CallID()
	var deviceId string

	if stream := StreamManager.RemoveWithCallId(id.Value()); stream != nil {
		// 下级设备挂断, 关闭流
		deviceId = stream.ID.DeviceID()
		stream.Close(false, true)
	} else if session := StreamManager.RemoveWithCallId(id.Value()); session != nil {
		// 广播挂断
		deviceId = session.ID.DeviceID()
		session.Close(false, true)
	}

	if parent {
		// 上级设备挂断
		if platform := PlatformManager.Find(req.Source()); platform != nil {
			platform.OnBye(req)
		}
	} else if device := DeviceManager.Find(deviceId); device != nil {
		device.OnBye(req)
	}
}

func (s *sipServer) OnNotify(req sip.Request, tx sip.ServerTransaction, parent bool) {
	response := sip.NewResponseFromRequest("", req, 200, "OK", "")
	SendResponse(tx, response)

	mobilePosition := MobilePositionNotify{}
	if err := DecodeXML([]byte(req.Body()), &mobilePosition); err != nil {
		Sugar.Errorf("解析位置通知失败 err: %s request: %s", err.Error(), req.String())
		return
	}

	if device := DeviceManager.Find(mobilePosition.DeviceID); device != nil {
		s.handler.OnNotifyPosition(&mobilePosition)
	}
}

func (s *sipServer) OnMessage(req sip.Request, tx sip.ServerTransaction, parent bool) {
	var ok bool
	defer func() {
		var response sip.Response
		if ok {
			response = CreateResponseWithStatusCode(req, http.StatusOK)
		} else {
			response = CreateResponseWithStatusCode(req, http.StatusForbidden)
		}

		SendResponse(tx, response)
	}()

	body := req.Body()
	xmlName := GetRootElementName(body)
	cmd := GetCmdType(body)
	src, ok := s.xmlReflectTypes[xmlName+"."+cmd]
	if !ok {
		Sugar.Errorf("处理XML消息失败, 找不到结构体. request: %s", req.String())
		return
	}

	message := reflect.New(src).Interface()
	if err := DecodeXML([]byte(body), message); err != nil {
		Sugar.Errorf("解析XML消息失败 err: %s request: %s", err.Error(), body)
		return
	}

	// 查找设备
	var device GBDevice
	deviceId := message.(BaseMessageGetter).GetDeviceID()
	if CmdBroadcast == cmd {
		// 广播消息
		from, _ := req.From()
		deviceId = from.Address.User().String()
	}
	if parent {
		device = PlatformManager.Find(req.Source())
	} else {
		device = DeviceManager.Find(deviceId)
	}

	if ok = device != nil; !ok {
		Sugar.Errorf("处理XML消息失败, 设备离线: %s request: %s", deviceId, req.String())
		return
	}

	switch xmlName {
	case XmlNameControl:
		break
	case XmlNameQuery:
		// 被上级查询
		var client GBClient
		client, ok = device.(GBClient)
		if !ok {
			Sugar.Errorf("处理XML消息失败, 类型转换失败. request: %s", req.String())
			return
		}

		if CmdDeviceInfo == cmd {
			client.OnQueryDeviceInfo(message.(*BaseMessage).SN)
		} else if CmdCatalog == cmd {
			var channels []*Channel

			// 查询出所有通道
			if DB != nil {
				result, err := DB.QueryPlatformChannels(client.(*GBPlatform).ServerAddr)
				if err != nil {
					Sugar.Errorf("查询设备通道列表失败 err: %s device: %s", err.Error(), client.GetID())
				}

				channels = result
			} else {
				// 从模拟多个国标客户端中查找
				channels = DeviceChannelsManager.FindChannels(client.GetID())
			}

			client.OnQueryCatalog(message.(*BaseMessage).SN, channels)
		}

		break
	case XmlNameNotify:
		if CmdKeepalive == cmd {
			// 下级设备心跳通知
			ok = s.handler.OnKeepAlive(deviceId)
		}

		break

	case XmlNameResponse:
		// 查询下级的应答
		if CmdCatalog == cmd {
			go s.handler.OnCatalog(device, message.(*CatalogResponse))
		} else if CmdRecordInfo == cmd {
			go s.handler.OnRecord(device, message.(*QueryRecordInfoResponse))
		} else if CmdDeviceInfo == cmd {
			go s.handler.OnDeviceInfo(device, message.(*DeviceInfoResponse))
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
func filterRequest(f func(req sip.Request, tx sip.ServerTransaction, parent bool)) gosip.RequestHandler {
	return func(req sip.Request, tx sip.ServerTransaction) {

		source := req.Source()
		platform := PlatformManager.Find(source)
		switch req.Method() {
		case sip.SUBSCRIBE, sip.INFO:
			if platform == nil {
				// SUBSCRIBE/INFO只能上级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
				Sugar.Errorf("处理%s请求失败, %s消息只能上级发起. request: %s", req.Method(), req.Method(), req.String())
				return
			}
			break
		case sip.NOTIFY, sip.REGISTER:
			if platform != nil {
				// NOTIFY和REGISTER只能下级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
				Sugar.Errorf("处理%s请求失败, %s消息只能下级发起. request: %s", req.Method(), req.Method(), req.String())
				return
			}
			break
		}

		f(req, tx, platform != nil)
	}
}

func StartSipServer(id, listenIP, publicIP string, listenPort int) (SipServer, error) {
	ua := gosip.NewServer(gosip.ServerConfig{
		Host: publicIP,
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

	utils.Assert(ua.OnRequest(sip.INFO, filterRequest(func(req sip.Request, tx sip.ServerTransaction, parent bool) {
	})) == nil)
	utils.Assert(ua.OnRequest(sip.CANCEL, filterRequest(func(req sip.Request, tx sip.ServerTransaction, parent bool) {
	})) == nil)
	utils.Assert(ua.OnRequest(sip.SUBSCRIBE, filterRequest(func(req sip.Request, tx sip.ServerTransaction, parent bool) {
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
