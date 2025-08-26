package stack

import (
	"encoding/json"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"sync/atomic"
)

type Stream struct {
	*dao.StreamModel
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
		packetParser := parser.NewPacketParser(common.Logger)
		message, err := packetParser.ParseMessage([]byte(v.Dialog))
		if err != nil {
			log.Sugar.Errorf("json解析dialog失败, err: %s value: %s", err.Error(), v.Dialog)
		} else {
			request := message.(sip.Request)
			s.SetDialog(request)
		}
	}

	return nil
}

func (s *Stream) GetSinkCount() int32 {
	return atomic.LoadInt32(&s.SinkCount)
}

func (s *Stream) IncreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, 1)
	//Sugar.Infof("拉流计数: %d stream: %s ", value, s.StreamID)
	// 启动协程去更新拉流计数, 可能会不一致
	//go Stream.SaveStream(s)
	return value
}

func (s *Stream) DecreaseSinkCount() int32 {
	value := atomic.AddInt32(&s.SinkCount, -1)
	//Sugar.Infof("拉流计数: %d stream: %s ", value, s.StreamID)
	//go Stream.SaveStream(s)
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
	_, _ = dao.Stream.DeleteStream(s.StreamID)
}

func (s *Stream) Bye() {
	if s.Dialog != nil && s.Dialog.Request != nil {
		go common.SipStack.SendRequest(s.CreateRequestFromDialog(sip.BYE))
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

func CloseStream(streamId common.StreamID, ms bool) {
	deleteStream, err := dao.Stream.DeleteStream(streamId)
	if err == nil {
		(&Stream{deleteStream}).Close(true, ms)
	}
}

// CloseStreamSinks 关闭某个流的所有sink
func CloseStreamSinks(StreamID common.StreamID, bye, ms bool) []*dao.SinkModel {
	sinks, _ := dao.Sink.DeleteForwardSinksByStreamID(StreamID)
	for _, sink := range sinks {
		(&Sink{sink}).Close(bye, ms)
	}

	return sinks
}
