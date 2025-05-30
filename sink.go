package main

import (
	"encoding/json"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
)

// Sink 国标级联转发流
type Sink struct {
	ID         string      `json:"id"`                 // 流媒体服务器中的sink id
	Stream     StreamID    `json:"stream"`             // 推流ID
	SinkStream StreamID    `json:"sink_stream"`        // 广播使用, 每个广播设备的唯一ID
	Protocol   string      `json:"protocol,omitempty"` // 转发流协议, gb_cascaded_forward/gb_talk_forward
	Dialog     sip.Request `json:"dialog,omitempty"`
	ServerAddr string      `json:"server_addr,omitempty"` // 级联上级地址
	CreateTime int64       `json:"create_time"`
	SetupType  SetupType   // 转发类型

	StreamWaiting
}

// Close 关闭级联会话. 是否向上级发送bye请求, 是否通知流媒体服务器发送删除sink
func (s *Sink) Close(bye, ms bool) {
	// 挂断与上级的sip会话
	if bye && s.Dialog != nil {
		byeRequest := CreateRequestFromDialog(s.Dialog, sip.BYE)
		go SipUA.SendRequest(byeRequest)
	}

	if ms {
		go CloseSink(string(s.Stream), s.ID)
	}
}

func (s *Sink) MarshalJSON() ([]byte, error) {
	type Alias Sink // 定义别名以避免递归调用
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

func (s *Sink) UnmarshalJSON(data []byte) error {
	type Alias Sink // 定义别名以避免递归调用
	v := &struct {
		*Alias
		Dialog string `json:"dialog,omitempty"` // 将 Dialog 转换为字符串
	}{
		Alias: (*Alias)(s),
	}

	if err := json.Unmarshal(data, v); err != nil {
		return err
	}

	*s = *(*Sink)(v.Alias)

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
