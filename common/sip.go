package common

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"github.com/ghettovoice/gosip/util"
)

var (
	Logger   log.Logger
	SipStack SipServer
)

func init() {
	Logger = log.NewDefaultLogrusLogger().WithPrefix("Server")
}

type SipServer interface {
	SendRequestWithContext(ctx context.Context,
		request sip.Request,
		options ...gosip.RequestWithContextOption)

	SendRequest(request sip.Request) sip.ClientTransaction

	Send(msg sip.Message) error

	ListenAddr() string

	Restart(id, listenIP, publicIP string, listenPort int) error

	Start(id, listenIP, publicIP string, listenPort int) error
}

// RequestWrapper sql序列化
type RequestWrapper struct {
	sip.Request
}

func (r *RequestWrapper) Value() (driver.Value, error) {
	if r == nil || r.Request == nil {
		return "", nil
	}

	return r.Request.String(), nil
}

func (r *RequestWrapper) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	data, ok := value.(string)
	if !ok {
		return errors.New("invalid type for RequestWrapper")
	} else if data == "" {
		return nil
	}

	dialog, err := UnmarshalDialog(data)
	if err != nil {
		return err
	}

	*r = RequestWrapper{dialog}
	return nil
}

func UnmarshalDialog(dialog string) (sip.Request, error) {
	packetParser := parser.NewPacketParser(Logger)
	message, err := packetParser.ParseMessage([]byte(dialog))
	if err != nil {
		return nil, err
	} else if request := message.(sip.Request); request == nil {
		return nil, fmt.Errorf("dialog message is not sip request")
	} else {
		return request, nil
	}
}

type SIPUAOptions struct {
	Name              string       `json:"name"`               // display name, 国标DeviceInfo消息中的Name
	Username          string       `json:"username"`           // 用户名
	ServerID          string       `json:"server_id"`          // 上级ID, 必选. 作为主键, 不能重复.
	ServerAddr        string       `json:"server_addr"`        // 上级地址, 必选
	Transport         string       `json:"transport"`          // 上级通信方式, UDP/TCP
	Password          string       `json:"password"`           // 密码
	RegisterExpires   int          `json:"register_expires"`   // 注册有效期
	KeepaliveInterval int          `json:"keepalive_interval"` // 心跳间隔
	Status            OnlineStatus `json:"status"`             // 在线状态
}

func SetToTag(response sip.Message) {
	toHeader := response.GetHeaders("To")
	to := toHeader[0].(*sip.ToHeader)
	to.Params = sip.NewParams().Add("tag", sip.String{Str: util.RandString(10)})
}

func SetHeader(msg sip.Message, header sip.Header) {
	msg.RemoveHeader(header.Name())
	msg.AppendHeader(header)
}

func SetHeaderIfNotExist(msg sip.Message, header sip.Header) {
	if len(msg.GetHeaders(header.Name())) < 1 {
		msg.AppendHeader(header)
	}
}
