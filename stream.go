package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"sync/atomic"
)

type SetupType int

const (
	SetupTypeUDP SetupType = iota
	SetupTypePassive
	SetupTypeActive
)

var (
	DefaultSetupType = SetupTypePassive
)

func (s SetupType) String() string {
	switch s {
	case SetupTypeUDP:
		return "udp"
	case SetupTypePassive:
		return "passive"
	case SetupTypeActive:
		return "active"
	}

	panic("invalid setup type")
}

func (s SetupType) MediaProtocol() string {
	switch s {
	case SetupTypePassive, SetupTypeActive:
		return "TCP/RTP/AVP"
	default:
		return "RTP/AVP"
	}
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

type Stream struct {
	GBModel
	StreamID  StreamID        `json:"stream_id"`          // 流ID
	Protocol  int             `json:"protocol,omitempty"` // 推流协议, rtmp/28181/1078/gb_talk
	Dialog    *RequestWrapper `json:"dialog,omitempty"`   // 国标流的SipCall会话
	SinkCount int32           `json:"sink_count"`         // 拉流端计数(包含级联转发)
	SetupType SetupType
	CallID    string `json:"call_id"`

	urls []string // 从流媒体服务器返回的拉流地址
}

func (s *Stream) MarshalJSON() ([]byte, error) {
	type Alias Stream // 定义别名以避免递归调用
	v := &struct {
		*Alias
		Dialog string `json:"dialog,omitempty"` // 将 Dialog 转换为字符串
	}{
		Alias: (*Alias)(s),
	}

	if s.Dialog != nil {
		v.Dialog = s.Dialog.String()
	}

	return json.Marshal(v)
}

func (s *Stream) UnmarshalJSON(data []byte) error {
	type Alias Stream // 定义别名以避免递归调用
	v := &struct {
		*Alias
		Dialog string `json:"dialog,omitempty"` // 将 Dialog 转换为字符串
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, v); err != nil {
		return err
	}

	*s = *(*Stream)(v.Alias)

	if len(v.Dialog) > 1 {
		packetParser := parser.NewPacketParser(logger)
		message, err := packetParser.ParseMessage([]byte(v.Dialog))
		if err != nil {
			Sugar.Errorf("json解析dialog失败, err: %s value: %s", err.Error(), v.Dialog)
		} else {
			request := message.(sip.Request)
			s.SetDialog(request)
		}
	}

	return nil
}

func (s *Stream) SetDialog(dialog sip.Request) {
	s.Dialog = &RequestWrapper{dialog}
	id, _ := dialog.CallID()
	s.CallID = id.Value()
}

func (s *Stream) GetSinkCount() int32 {
	return atomic.LoadInt32(&s.SinkCount)
}

func (s *Stream) IncreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, 1)
	//Sugar.Infof("拉流计数: %d stream: %s ", value, s.StreamID)
	// 启动协程去更新拉流计数, 可能会不一致
	//go StreamDao.SaveStream(s)
	return value
}

func (s *Stream) DecreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, -1)
	//Sugar.Infof("拉流计数: %d stream: %s ", value, s.StreamID)
	//go StreamDao.SaveStream(s)
	return value
}

func (s *Stream) Close(bye, ms bool) {
	// 断开与推流通道的sip会话
	if bye {
		s.Bye()
	}

	if ms {
		// 告知媒体服务释放source
		go MSCloseSource(string(s.StreamID))
	}

	// 关闭所转发会话
	CloseStreamSinks(s.StreamID, bye, ms)

	// 从数据库中删除流记录
	_, _ = StreamDao.DeleteStream(s.StreamID)
}

func (s *Stream) Bye() {
	if s.Dialog != nil && s.Dialog.Request != nil {
		go SipStack.SendRequest(s.CreateRequestFromDialog(sip.BYE))
		s.Dialog = nil
	}
}

func CreateRequestFromDialog(dialog sip.Request, method sip.RequestMethod) sip.Request {
	{
		seq, _ := dialog.CSeq()
		seq.SeqNo++
		seq.MethodName = method
	}

	request := dialog.Clone().(sip.Request)
	request.SetMethod(method)
	request.SetSource("")
	return request
}

func (s *Stream) CreateRequestFromDialog(method sip.RequestMethod) sip.Request {
	return CreateRequestFromDialog(s.Dialog, method)
}

func CloseStream(streamId StreamID, ms bool) {
	deleteStream, err := StreamDao.DeleteStream(streamId)
	if err == nil {
		deleteStream.Close(true, ms)
	}
}
