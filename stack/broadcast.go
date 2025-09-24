package stack

import (
	"context"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/utils"
	"net/http"
	"time"
)

const (
	BroadcastFormat = "<?xml version=\"1.0\" encoding=\"GB2312\" ?>\r\n" +
		"<Notify>\r\n" +
		"<CmdType>Broadcast</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<SourceID>%s</SourceID>\r\n" +
		"<TargetID>%s</TargetID>\r\n" +
		"</Notify>\r\n"
)

func (d *Device) StartBroadcast(streamId common.StreamID, deviceId, channelId string, timeoutCtx context.Context) (*dao.SinkModel, error) {
	// 生成sinkstreamid, 该通道的唯一广播id
	sinkStreamId := common.GenerateStreamID(common.InviteTypeBroadcast, deviceId, channelId, "", "")
	// 生成source id, 关联会话. 下发broadcast message告知设备, 设备的invite请求行将携带
	inviteSourceId := utils.RandStringBytes(20)

	var ok bool
	defer func() {
		EarlyDialogs.Remove(inviteSourceId)
		EarlyDialogs.Remove(d.DeviceID)
		if !ok {
			_, _ = dao.Sink.DeleteSinkBySinkStreamID(sinkStreamId)
		}
	}()

	sink := &dao.SinkModel{
		SinkStreamID: sinkStreamId,
		StreamID:     streamId,
		Protocol:     TransStreamGBTalk,
		CreateTime:   time.Now().Unix(),
		SetupType:    common.SetupTypePassive,
	}

	// 保存sink, 保存失败认为该设备正在广播
	if err := dao.Sink.CreateSink(sink); err != nil {
		return nil, err
	}

	// 查找音频输出通道
	var audioChannelId = channelId
	if subChannels, err := dao.Channel.QueryChannelsByParentID(deviceId, channelId); err == nil {
		for _, channel := range subChannels {
			if 137 != channel.TypeCode {
				continue
			}

			audioChannelId = channel.DeviceID
			break
		}
	}

	// 关联会话
	streamWaiting := &StreamWaiting{Data: sink}
	if _, ok = EarlyDialogs.Add(inviteSourceId, streamWaiting); !ok {
		return nil, fmt.Errorf("id冲突")
	} else if _, ok = EarlyDialogs.Add(d.DeviceID, streamWaiting); !ok {
		// 使用设备ID关联下会话, 兼容不标准的下级设备. 如果下级设备都不标准，意味着同时只能对一个通道发起对讲.
	}

	// 信令交互
	transaction := d.Broadcast(inviteSourceId, audioChannelId)
	responses := transaction.Responses()
	select {
	// 等待message broadcast的应答
	case response := <-responses:
		if response == nil {
			return nil, fmt.Errorf("信令超时")
		}

		if response.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("错误响应 code: %d", response.StatusCode())
		}

		// 等待下级设备的Invite请求
		code := streamWaiting.Receive(10)
		if code == -1 {
			return nil, fmt.Errorf("等待invite超时")
		} else if http.StatusOK != code {
			return nil, fmt.Errorf("错误应答 code: %d", code)
		} else {
			ok = true
			return sink, nil
		}
	case <-timeoutCtx.Done():
		// 外部调用超时
		streamWaiting.Put(-1)
		break
	}

	return nil, fmt.Errorf("广播失败")
}

// OnInvite 收到设备的语音广播offer
func (d *Device) OnInvite(request sip.Request, user string) sip.Response {
	// 查找会话, 先用source id查找, 找不到再根据设备id查找
	streamWaiting := EarlyDialogs.Find(user)
	if streamWaiting != nil {
		if streamWaiting = EarlyDialogs.Find(d.DeviceID); streamWaiting == nil {
			return CreateResponseWithStatusCode(request, http.StatusBadRequest)
		}
	}

	// 解析offer
	sink := streamWaiting.Data.(*dao.SinkModel)
	body := request.Body()
	offer, err := ParseGBSDP(body)
	if err != nil {
		log.Sugar.Infof("广播失败, 解析sdp发生err: %s  sink: %s  sdp: %s", err.Error(), sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	} else if offer.Media == nil {
		log.Sugar.Infof("广播失败, offer中缺少audio字段. sink: %s sdp: %s", sink.SinkID, body)
		streamWaiting.Put(http.StatusBadRequest)
		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
	}

	// http接口中设置的setup优先级高于sdp中的setup
	if offer.AnswerSetup != sink.SetupType {
		offer.AnswerSetup = sink.SetupType
	}

	// 添加sink到流媒体服务器
	response, err := AddForwardSink(TransStreamGBTalk, request, user, &Sink{sink}, sink.StreamID, offer, common.InviteTypeBroadcast, "8 PCMA/8000")
	if err != nil {
		log.Sugar.Errorf("广播失败, 流媒体创建answer发生err: %s  sink: %s ", err.Error(), sink.SinkID)
		streamWaiting.Put(http.StatusInternalServerError)
		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
	}

	streamWaiting.Put(http.StatusOK)
	return response
}
