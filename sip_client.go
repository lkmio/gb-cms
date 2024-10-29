package main

import (
	"context"
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"math"
	"net"
	"strconv"
	"time"
)

const (
	KeepAliveBody = "<?xml version=\"1.0\" encoding=\"GB2312\"?>\r\n" +
		"<Notify>\r\n" +
		"<CmdType>Keepalive</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<Status>OK</Status>\r\n" +
		"</Notify>\r\n"
)

var (
	UnregisterExpiresHeader = sip.Expires(0)
)

type SipClient interface {
	doRegister(request sip.Request) bool

	doUnregister()

	doKeepalive() bool

	Start()

	Stop()

	SetOnRegisterHandler(online, offline func())
}

type sipClient struct {
	Username         string
	Domain           string //注册域
	Transport        string
	Password         string //密码
	RegisterExpires  int    //注册有效期
	KeeAliveInterval int    //心跳间隔
	SeverId          string //上级ID

	ListenAddr string //UA的监听地址
	NatAddr    string //Nat地址

	ua                   SipServer
	exited               bool
	ctx                  context.Context
	cancel               context.CancelFunc
	keepaliveFailedCount int
	registerOK           bool
	registerOKTime       time.Time //注册成功时间
	registerOKRequest    sip.Request

	onlineCB  func()
	offlineCB func()
}

func (g *sipClient) doRegister(request sip.Request) bool {
	hop, _ := request.ViaHop()
	empty := sip.String{}
	hop.Params.Add("rport", &empty)
	hop.Params.Add("received", &empty)

	for i := 0; i < 2; i++ {
		//发起注册, 第一次未携带授权头, 第二次携带授权头
		clientTransaction := g.ua.SendRequest(request)

		//等待响应
		responses := clientTransaction.Responses()
		var response sip.Response
		select {
		case response = <-responses:
			break
		case <-g.ctx.Done():
			break
		}

		if response == nil {
			break
		} else if response.StatusCode() == 200 {
			g.registerOKRequest = request.Clone().(sip.Request)
			viaHop, _ := response.ViaHop()
			rport, _ := viaHop.Params.Get("rport")
			received, _ := viaHop.Params.Get("received")
			if rport != nil && received != nil {
				g.NatAddr = net.JoinHostPort(received.String(), rport.String())
			}
			return true
		} else if response.StatusCode() == 401 || response.StatusCode() == 407 {
			authorizer := sip.DefaultAuthorizer{Password: sip.String{Str: g.Password}, User: sip.String{Str: g.Username}}
			if err := authorizer.AuthorizeRequest(request, response); err != nil {
				break
			}
		} else {
			break
		}
	}

	return false
}

func (g *sipClient) startNewRegister() bool {
	builder := NewRequestBuilder(sip.REGISTER, g.Username, g.ListenAddr, g.SeverId, g.Domain, g.Transport)
	expires := sip.Expires(g.RegisterExpires)
	builder.SetExpires(&expires)

	host, p, _ := net.SplitHostPort(g.ListenAddr)
	port, _ := strconv.Atoi(p)
	sipPort := sip.Port(port)
	builder.SetTo(&sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: g.Username},
			FHost: host,
			FPort: &sipPort,
		},
	})

	request, err := builder.Build()
	if err != nil {
		panic(err)
	}

	if ok := g.doRegister(request); ok {
		g.registerOKRequest = request
		return ok
	}

	return false
}

func CopySipRequest(old sip.Request) sip.Request {
	//累加cseq number
	cseq, _ := old.CSeq()
	cseq.SeqNo++

	request := old.Clone().(sip.Request)
	//清空事务标记
	hop, _ := request.ViaHop()
	hop.Params.Remove("branch")
	return request
}

func (g *sipClient) refreshRegister() bool {
	request := CopySipRequest(g.registerOKRequest)
	return g.doRegister(request)
}

func (g *sipClient) doUnregister() {
	request := CopySipRequest(g.registerOKRequest)
	request.RemoveHeader("Expires")
	request.AppendHeader(&UnregisterExpiresHeader)
	g.ua.SendRequest(request)

	if g.offlineCB != nil {
		go g.offlineCB()
	}
}

func (g *sipClient) doKeepalive() bool {
	body := fmt.Sprintf(KeepAliveBody, time.Now().UnixMilli()/1000, g.Username)
	request, err := BuildMessageRequest(g.Username, g.ListenAddr, g.SeverId, g.Domain, g.Transport, body)
	if err != nil {
		panic(err)
	}

	transaction := g.ua.SendRequest(request)
	responses := transaction.Responses()

	var response sip.Response
	select {
	case response = <-responses:
		break
	case <-g.ctx.Done():
		break
	}

	return response != nil && response.StatusCode() == 200
}

// IsExpires 是否临近注册有效期
func (g *sipClient) IsExpires() (bool, int) {
	if !g.registerOK {
		return false, 0
	}

	dis := g.RegisterExpires - int(time.Now().Sub(g.registerOKTime).Seconds())
	return dis <= 10, dis - 10
}

// Refresh 处理Client的生命周期任务, 发起注册, 发送心跳，断开重连等， 并返回下次刷新任务时间
func (g *sipClient) Refresh() time.Duration {
	expires, _ := g.IsExpires()

	if !g.registerOK || expires {
		if expires {
			g.registerOK = g.refreshRegister()
		} else {
			g.registerOK = g.startNewRegister()
		}

		if g.registerOK {
			g.registerOKTime = time.Now()
			if g.onlineCB != nil {
				go g.onlineCB()
			}
		}
	}

	// 注册失败后, 等待10秒钟再发起注册
	if !g.registerOK {
		return 10 * time.Second
	}

	// 发送心跳
	if !g.doKeepalive() {
		g.keepaliveFailedCount++
	} else {
		g.keepaliveFailedCount = 0
	}

	// 心跳失败, 重新发起注册
	if g.keepaliveFailedCount > 0 {
		g.keepaliveFailedCount = 0
		g.registerOK = false
		g.registerOKRequest = nil
		g.NatAddr = ""

		if g.offlineCB != nil {
			go g.offlineCB()
		}

		// 立马发起注册
		return 0
	}

	// 信令正常, 休眠心跳间隔时长
	return time.Duration(g.KeeAliveInterval) * time.Second
}

func (g *sipClient) Start() {
	utils.Assert(!g.exited)
	g.ctx, g.cancel = context.WithCancel(context.Background())

	go func() {
		for !g.exited {
			duration := g.Refresh()
			expires, dis := g.IsExpires()
			if duration < time.Second || expires {
				continue
			} else if g.registerOK {
				duration = time.Duration(int(math.Min(duration.Seconds(), float64(dis)))) * time.Second
			}

			if g.exited {
				return
			}

			select {
			case <-time.After(duration):
				break
			case <-g.ctx.Done():
				break
			}
		}
	}()
}

func (g *sipClient) Stop() {
	utils.Assert(!g.exited)

	g.exited = true
	g.cancel()
	g.registerOK = false
	g.onlineCB = nil
	g.offlineCB = nil

	if g.registerOK {
		g.doUnregister()
	}
}

func (g *sipClient) SetOnRegisterHandler(online, offline func()) {
	g.onlineCB = online
	g.offlineCB = offline
}
