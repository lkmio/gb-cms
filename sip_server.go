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
	var device *Device
	var query bool
	_ = req.GetHeaders("Authorization")
	fromHeader := req.GetHeaders("From")[0].(*sip.FromHeader)
	expiresHeader := req.GetHeaders("Expires")

	response := sip.NewResponseFromRequest("", req, 200, "OK", "")

	if expiresHeader != nil && "0" == expiresHeader[0].Value() {
		Sugar.Infof("注销信令 from:%s", fromHeader.Address.User())
		DB.UnRegisterDevice(fromHeader.Name())
	} else /*if authorizationHeader == nil*/ {
		expires := sip.Expires(3600)
		response.AppendHeader(&expires)

		//sip.NewResponseFromRequest("", req, 401, "Unauthorized", "")

		device = &Device{
			ID:         fromHeader.Address.User().String(),
			Transport:  req.Transport(),
			RemoteAddr: req.Source(),
		}

		err, b := DB.RegisterDevice(device)
		query = err != nil || b
	}

	SendResponse(tx, response)

	if device != nil && query {
		device.QueryCatalog()
	}
}

// OnInvite 上级预览/下级广播
func (s *sipServer) OnInvite(req sip.Request, tx sip.ServerTransaction, parent bool) {
	SendResponse(tx, sip.NewResponseFromRequest("", req, 100, "Trying", ""))
	user := req.Recipient().User().String()

	if len(user) != 20 {
		SendResponseWithStatusCode(req, tx, http.StatusNotFound)
		return
	}

	// 查找对应的设备
	var device GBDevice
	if parent {
		// 级联设备
		device = PlatformManager.FindPlatformWithServerAddr(req.Source())
	} else if session := FindBroadcastSessionWithSourceID(user); session != nil {
		// 语音广播设备
		device = DeviceManager.Find(session.DeviceID)
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
		stream.Close(false)
	} else if session := BroadcastManager.RemoveWithCallId(id.Value()); session != nil {
		// 广播挂断
		deviceId = session.DeviceID
		session.Close(false)
	}

	if parent {
		// 上级设备挂断
		if platform := PlatformManager.FindPlatformWithServerAddr(req.Source()); platform != nil {
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
		Sugar.Errorf("解析位置通知失败 err:%s body:%s", err.Error(), req.Body())
		return
	}

	if device := DeviceManager.Find(mobilePosition.DeviceID); device != nil {
		device.OnNotifyPosition(&mobilePosition)
	}
}

func (s *sipServer) OnMessage(req sip.Request, tx sip.ServerTransaction, parent bool) {
	var online bool
	defer func() {
		var response sip.Response
		if online {
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
		return
	}

	message := reflect.New(src).Interface()
	if err := DecodeXML([]byte(body), message); err != nil {
		Sugar.Errorf("解析xml异常 >>> %s %s", err.Error(), body)
		return
	}

	// 查找设备
	var device GBDevice
	deviceId := message.(BaseMessageGetter).GetDeviceID()
	if parent {
		device = PlatformManager.FindPlatformWithServerAddr(req.Source())
	} else {
		device = DeviceManager.Find(deviceId)
	}

	if online = device != nil; !online {
		Sugar.Errorf("处理Msg失败 设备离线: %s Msg: %s", deviceId, body)
		return
	}

	switch xmlName {
	case XmlNameControl:
		break
	case XmlNameQuery:
		client, ok := device.(GBClient)
		if !ok {
			online = false
			return
		}

		if CmdDeviceInfo == cmd {
			client.OnQueryDeviceInfo(message.(*BaseMessage).SN)
		} else if CmdCatalog == cmd {
			client.OnQueryCatalog(message.(*BaseMessage).SN)
		}
		break
	case XmlNameNotify:
		if CmdKeepalive == cmd {
			device.OnKeepalive()
		}
		break
	case XmlNameResponse:
		if CmdCatalog == cmd {
			device.OnCatalog(message.(*CatalogResponse))
		} else if CmdRecordInfo == cmd {
			device.OnRecord(message.(*QueryRecordInfoResponse))
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
	Sugar.Infof("send response >>> %s", response.String())
	sendError := tx.Respond(response)

	if sendError != nil {
		Sugar.Infof("send response error %s %s", sendError.Error(), response.String())
	}

	return sendError == nil
}

func (s *sipServer) SendRequestWithContext(ctx context.Context, request sip.Request, options ...gosip.RequestWithContextOption) {
	Sugar.Infof("send reqeust: %s", request.String())
	s.sip.RequestWithContext(ctx, request, options...)
}

func (s *sipServer) SendRequestWithTimeout(seconds int, request sip.Request, options ...gosip.RequestWithContextOption) (sip.Response, error) {
	Sugar.Infof("send reqeust: %s", request.String())
	reqCtx, _ := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	return s.sip.RequestWithContext(reqCtx, request, options...)
}

func (s *sipServer) SendRequest(request sip.Request) sip.ClientTransaction {
	Sugar.Infof("send reqeust: %s", request.String())
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
		Sugar.Infof("process request: %s", req.String())

		source := req.Source()
		platform := PlatformManager.FindPlatformWithServerAddr(source)
		switch req.Method() {
		case sip.SUBSCRIBE, sip.INFO:
			if platform == nil {
				// SUBSCRIBE/INFO只能上级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
				return
			}
			break
		case sip.NOTIFY, sip.REGISTER:
			if platform != nil {
				// NOTIFY和REGISTER只能下级发起
				SendResponseWithStatusCode(req, tx, http.StatusBadRequest)
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
