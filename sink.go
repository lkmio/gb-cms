package main

import (
	"encoding/json"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"net/http"
	"net/url"
)

// Sink 级联/对讲/网关转发流Sink
type Sink struct {
	GBModel
	SinkID       string          `json:"sink_id"`            // 流媒体服务器中的sink id
	StreamID     StreamID        `json:"stream_id"`          // 推流ID
	SinkStreamID StreamID        `json:"sink_stream_id"`     // 广播使用, 每个广播设备的唯一ID
	Protocol     string          `json:"protocol,omitempty"` // 转发流协议, gb_cascaded/gb_talk/gb_gateway
	Dialog       *RequestWrapper `json:"dialog,omitempty"`
	CallID       string          `json:"call_id,omitempty"`
	ServerAddr   string          `json:"server_addr,omitempty"` // 级联上级地址
	CreateTime   int64           `json:"create_time"`
	SetupType    SetupType       // 流转发类型
}

// Close 关闭级联会话. 是否向上级发送bye请求, 是否通知流媒体服务器发送删除sink
func (s *Sink) Close(bye, ms bool) {
	// 挂断与上级的sip会话
	if bye {
		s.Bye()
	}

	if ms {
		go MSCloseSink(string(s.StreamID), s.SinkID)
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

func (s *Sink) Bye() {
	if s.Dialog != nil && s.Dialog.Request != nil {
		byeRequest := CreateRequestFromDialog(s.Dialog.Request, sip.BYE)
		go SipStack.SendRequest(byeRequest)
	}
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
			s.SetDialog(request)
		}
	}

	return nil
}

func (s *Sink) SetDialog(dialog sip.Request) {
	s.Dialog = &RequestWrapper{dialog}
	id, _ := dialog.CallID()
	s.CallID = id.Value()
}

// AddForwardSink 向流媒体服务添加转发Sink
func AddForwardSink(forwardType int, request sip.Request, user string, sink *Sink, streamId StreamID, gbSdp *GBSDP, inviteType InviteType, attrs ...string) (sip.Response, error) {
	urlParams := make(url.Values)
	if TransStreamGBTalk == forwardType {
		urlParams.Add("forward_type", "broadcast")
	} else if TransStreamGBCascaded == forwardType {
		urlParams.Add("forward_type", "cascaded")
	} else if TransStreamGBGateway == forwardType {
		urlParams.Add("forward_type", "gateway_1078")
	}

	ip, port, sinkID, ssrc, err := MSAddForwardSink(forwardType, string(streamId), gbSdp.connectionAddr, gbSdp.offerSetup.String(), gbSdp.answerSetup.String(), gbSdp.ssrc, string(inviteType), urlParams)
	if err != nil {
		Sugar.Errorf("处理上级Invite失败,向流媒体服务添加转发Sink失败 err: %s", err.Error())
		if InviteTypePlay != inviteType {
			CloseStream(streamId, true)
		}

		return nil, err
	}

	sink.SinkID = sinkID
	// 创建answer
	answer := BuildSDP(gbSdp.mediaType, user, gbSdp.sdp.Session, ip, port, gbSdp.startTime, gbSdp.stopTime, gbSdp.answerSetup.String(), gbSdp.speed, ssrc, attrs...)
	response := CreateResponseWithStatusCode(request, http.StatusOK)

	// answer添加contact头域
	response.RemoveHeader("Contact")
	response.AppendHeader(GlobalContactAddress.AsContactHeader())
	response.AppendHeader(&SDPMessageType)
	response.SetBody(answer, true)
	setToTag(response)

	sink.SetDialog(CreateDialogRequestFromAnswer(response, true, request.Source()))

	if err = SinkDao.SaveForwardSink(streamId, sink); err != nil {
		Sugar.Errorf("保存sink到数据库失败, stream: %s sink: %s err: %s", streamId, sink.SinkID, err.Error())
	}

	return response, nil
}
