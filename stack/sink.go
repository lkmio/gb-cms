package stack

import (
	"encoding/json"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"net/http"
	"net/url"
)

// Sink 级联/对讲/网关转发流Sink
type Sink struct {
	*dao.SinkModel
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

	// 目前只有一对一对讲, 断开就删除整个websocket对讲流
	if s.Protocol == TransStreamGBTalk {
		_, _ = dao.Sink.DeleteSinkBySinkStreamID(s.SinkStreamID)
		_, _ = dao.Stream.DeleteStream(s.StreamID)
		// 删除流媒体source
		_ = MSCloseSource(string(s.StreamID))
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
		byeRequest := CreateRequestFromDialog(s.Dialog.Request, sip.BYE, "", 0)
		go common.SipStack.SendRequest(byeRequest)
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

func (s *Sink) SetDialog(dialog sip.Request) {
	s.Dialog = &common.RequestWrapper{Request: dialog}
	id, _ := dialog.CallID()
	s.CallID = id.Value()
}

// AddForwardSink 向流媒体服务添加转发Sink
func AddForwardSink(forwardType int, request sip.Request, user string, sink *Sink, streamId common.StreamID, gbSdp *GBSDP, inviteType common.InviteType, attrs ...string) (sip.Response, error) {
	urlParams := make(url.Values)
	if TransStreamGBTalk == forwardType {
		urlParams.Add("forward_type", "broadcast")
	} else if TransStreamGBCascaded == forwardType {
		urlParams.Add("forward_type", "cascaded")
	} else if TransStreamGBGateway == forwardType {
		urlParams.Add("forward_type", "gateway_1078")
	}

	ip, port, sinkID, ssrc, err := MSAddForwardSink(forwardType, string(streamId), gbSdp.ConnectionAddr, gbSdp.OfferSetup.String(), gbSdp.AnswerSetup.String(), gbSdp.SSRC, string(inviteType), urlParams)
	if err != nil {
		log.Sugar.Errorf("处理上级Invite失败,向流媒体服务添加转发Sink失败 err: %s", err.Error())
		if common.InviteTypePlay != inviteType {
			CloseStream(streamId, true)
		}

		return nil, err
	}

	sink.SinkID = sinkID
	// 创建answer
	answer := BuildSDP(gbSdp.MediaType, user, gbSdp.SDP.Session, ip, port, gbSdp.StartTime, gbSdp.StopTime, gbSdp.AnswerSetup.String(), gbSdp.Speed, ssrc, attrs...)
	response := CreateResponseWithStatusCode(request, http.StatusOK)

	// answer添加contact头域
	common.SetHeader(response, GlobalContactAddress.AsContactHeader())
	common.SetHeader(response, &SDPMessageType)

	response.SetBody(answer, true)
	common.SetToTag(response)

	sink.SetDialog(CreateDialogRequestFromAnswer(response, true, request.Source()))

	if err = dao.Sink.SaveSink(sink.SinkModel); err != nil {
		log.Sugar.Errorf("保存sink到数据库失败, stream: %s sink: %s err: %s", streamId, sink.SinkID, err.Error())
	}

	return response, nil
}
