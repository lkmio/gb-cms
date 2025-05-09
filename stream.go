package main

import (
	"context"
	"encoding/json"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"sync/atomic"
	"time"
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

type StreamWaiting struct {
	onPublishCb chan int // 等待推流hook的管道
	cancelFunc  func()   // 取消等待推流hook的ctx
}

func (s *StreamWaiting) WaitForPublishEvent(seconds int) int {
	s.onPublishCb = make(chan int, 0)
	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	s.cancelFunc = cancelFunc
	select {
	case code := <-s.onPublishCb:
		return code
	case <-timeout.Done():
		s.cancelFunc = nil
		return -1
	}
}

type Stream struct {
	ID         StreamID    `json:"id"`                 // 流ID
	Protocol   string      `json:"protocol,omitempty"` // 推流协议, rtmp/28181/1078/gb_talk
	Dialog     sip.Request `json:"dialog,omitempty"`   // 国标流的SipCall会话
	CreateTime int64       `json:"create_time"`        // 推流时间
	SinkCount  int32       `json:"sink_count"`         // 拉流端计数(包含级联转发)
	SetupType  SetupType

	urls []string // 从流媒体服务器返回的拉流地址
	StreamWaiting
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
			s.Dialog = request
		}
	}

	return nil
}

func (s *Stream) GetSinkCount() int32 {
	return atomic.LoadInt32(&s.SinkCount)
}

func (s *Stream) IncreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, 1)
	Sugar.Infof("拉流计数: %d stream: %s ", value, s.ID)
	// 启动协程去更新拉流计数, 可能会不一致
	go DB.SaveStream(s)
	return value
}

func (s *Stream) DecreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, -1)
	Sugar.Infof("拉流计数: %d stream: %s ", value, s.ID)
	go DB.SaveStream(s)
	return value
}

func (s *Stream) Close(bye, ms bool) {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// 断开与推流通道的sip会话
	if bye && s.Dialog != nil {
		go SipUA.SendRequest(s.CreateRequestFromDialog(sip.BYE))
		s.Dialog = nil
	}

	if ms {
		// 告知媒体服务释放source
		go CloseSource(string(s.ID))
	}

	// 关闭所转发会话
	CloseStreamSinks(s.ID, bye, ms)

	// 从数据库中删除流记录
	DB.DeleteStream(s.CreateTime)
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
	stream := StreamManager.Remove(streamId)
	if stream != nil {
		stream.Close(true, ms)
	}
}
