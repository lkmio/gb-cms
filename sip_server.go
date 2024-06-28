package main

import (
	"context"
	"encoding/xml"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	logger               log.Logger
	globalContactAddress *sip.Address
)

const (
	CmdTagStart = "<CmdType>"
	CmdTagEnd   = "</CmdType>"
)

func init() {
	logger = log.NewDefaultLogrusLogger().WithPrefix("Server")
}

type SipServer interface {
	OnRegister(req sip.Request, tx sip.ServerTransaction)

	OnInvite(req sip.Request, tx sip.ServerTransaction)

	OnAck(req sip.Request, tx sip.ServerTransaction)

	OnBye(req sip.Request, tx sip.ServerTransaction)

	OnNotify(req sip.Request, tx sip.ServerTransaction)

	SendRequestWithContext(ctx context.Context,
		request sip.Request,
		options ...gosip.RequestWithContextOption)

	SendRequest(request sip.Request)

	SendRequestWithTimeout(seconds int, request sip.Request, options ...gosip.RequestWithContextOption) (sip.Response, error)

	Send(msg sip.Message) error
}

type sipServer struct {
	sip    gosip.Server
	db     DeviceDB
	config *Config_
}

func (s *sipServer) Send(msg sip.Message) error {
	return s.sip.Send(msg)
}

func (s *sipServer) OnRegister(req sip.Request, tx sip.ServerTransaction) {
	var device *DBDevice
	_ = req.GetHeaders("Authorization")
	fromHeader := req.GetHeaders("From")[0].(*sip.FromHeader)
	expiresHeader := req.GetHeaders("Expires")

	response := sip.NewResponseFromRequest("", req, 200, "OK", "")

	if expiresHeader != nil && "0" == expiresHeader[0].Value() {
		Sugar.Infof("注销信令 from:%s", fromHeader.Address.User())
		s.db.RemoveDevice(fromHeader.Name())
	} else /*if authorizationHeader == nil*/ {
		expires := sip.Expires(3600)
		response.AppendHeader(&expires)

		//sip.NewResponseFromRequest("", req, 401, "Unauthorized", "")
		device = &DBDevice{
			Id:         fromHeader.Address.User().String(),
			RemoteAddr: req.Source(),
		}

		s.db.AddDevice(device)
	}

	sendResponse(tx, response)

	if device != nil {
		catalog, err := device.BuildCatalogRequest()
		if err != nil {
			panic(err)
		}
		s.SendRequest(catalog)
	}
}

func (s *sipServer) OnInvite(req sip.Request, tx sip.ServerTransaction) {
}

func (s *sipServer) OnAck(req sip.Request, tx sip.ServerTransaction) {
}

func (s *sipServer) OnBye(req sip.Request, tx sip.ServerTransaction) {
}

func (s *sipServer) OnNotify(req sip.Request, tx sip.ServerTransaction) {
	response := sip.NewResponseFromRequest("", req, 200, "OK", "")
	sendResponse(tx, response)

	body := strings.Replace(req.Body(), "GB2312", "UTF-8", 1)
	mobilePosition := MobilePositionNotify{}
	if err := xml.Unmarshal([]byte(body), &mobilePosition); err != nil {
		Sugar.Errorf("解析位置通知失败 err:%s body:%s", err.Error(), body)
		return
	}

	if device := DeviceManager.Find(mobilePosition.DeviceID); device != nil {
		device.OnMobilePositionNotify(&mobilePosition)
	}
}

func sendResponse(tx sip.ServerTransaction, response sip.Response) bool {
	sendError := tx.Respond(response)
	Sugar.Infof("send response >>> %s", response.String())
	if sendError != nil {
		Sugar.Infof("send response error %s %s", sendError.Error(), response.String())
	}

	return sendError == nil
}

func (s *sipServer) SendRequestWithContext(ctx context.Context, request sip.Request, options ...gosip.RequestWithContextOption) {
	Sugar.Infof("send reqeust:%s", request.String())
	s.sip.RequestWithContext(ctx, request, options...)
}

func (s *sipServer) SendRequestWithTimeout(seconds int, request sip.Request, options ...gosip.RequestWithContextOption) (sip.Response, error) {
	Sugar.Infof("send reqeust:%s", request.String())
	reqCtx, _ := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	return s.sip.RequestWithContext(reqCtx, request, options...)
}

func (s *sipServer) SendRequest(request sip.Request) {
	Sugar.Infof("send reqeust:%s", request.String())

	clientTransaction, err := s.sip.Request(request)
	if err != nil {
		panic(err)
	}

	defer func() {
		if clientTransaction != nil {
			err = clientTransaction.Cancel()
		}
	}()
}

func StartSipServer(config *Config_, db DeviceDB) (SipServer, error) {
	server := gosip.NewServer(gosip.ServerConfig{
		Host: config.PublicIP,
	}, nil, nil, logger)

	err := server.Listen("udp", config.SipAddr)
	if err != nil {
		return nil, err
	}

	s := &sipServer{sip: server}
	server.OnRequest(sip.REGISTER, s.OnRegister)
	server.OnRequest(sip.INVITE, s.OnInvite)
	server.OnRequest(sip.BYE, s.OnBye)
	server.OnRequest(sip.ACK, s.OnAck)
	server.OnRequest(sip.NOTIFY, s.OnNotify)
	server.OnRequest(sip.MESSAGE, func(req sip.Request, tx sip.ServerTransaction) {
		response := sip.NewResponseFromRequest("", req, 200, "OK", "")
		sendResponse(tx, response)

		body := req.Body()
		startIndex := strings.Index(body, CmdTagStart)
		endIndex := strings.Index(body, CmdTagEnd)
		if startIndex <= 0 || endIndex <= 0 || endIndex+len(CmdTagStart) <= startIndex {
			Sugar.Warnf("未知消息 %s", body)
			return
		}

		cmd := strings.ToLower(body[startIndex+len(CmdTagStart) : endIndex])
		var message interface{}
		if "keepalive" == cmd {
			return
		} else if "catalog" == cmd {
			message = &QueryCatalogResponse{}
		} else if "recordinfo" == cmd {
			message = &QueryRecordInfoResponse{}
		} else if "mediastatus" == cmd {
			return
		}

		if err := DecodeXML([]byte(body), message); err != nil {
			Sugar.Errorf("解析xml异常 >>> %s %s", err.Error(), body)
			return
		}

		switch cmd {
		case "catalog":
			{
				if device := DeviceManager.Find(message.(*QueryCatalogResponse).DeviceID); device != nil {
					device.OnCatalog(message.(*QueryCatalogResponse))
				}
			}
			break

		case "recordinfo":
			{
				if device := DeviceManager.Find(message.(*QueryRecordInfoResponse).DeviceID); device != nil {
					device.OnRecord(message.(*QueryRecordInfoResponse))
				}
			}
			break
		}
	})

	s.db = db
	s.config = config

	_, p, _ := net.SplitHostPort(config.SipAddr)
	port, _ := strconv.Atoi(p)
	config.SipPort = sip.Port(port)

	globalContactAddress = &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: config.SipId},
			FHost: config.PublicIP,
			FPort: &config.SipPort,
		},
	}

	return s, err
}
