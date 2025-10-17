package api

import (
	"context"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

func (api *ApiServer) OnHangup(v *BroadcastParams, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	log.Sugar.Debugf("广播挂断 %v", *v)

	id := common.GenerateStreamID(common.InviteTypeBroadcast, v.DeviceID, v.ChannelID, "", "")
	if sink, _ := dao.Sink.DeleteSinkBySinkStreamID(id); sink != nil {
		(&stack.Sink{SinkModel: sink}).Close(true, true)
	}

	return nil, nil
}

func (api *ApiServer) OnBroadcast(v *BroadcastParams, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	log.Sugar.Debugf("广播邀请 %v", *v)

	model, _ := dao.Device.QueryDevice(v.DeviceID)
	if model == nil || !model.Online() {
		return nil, fmt.Errorf("设备离线")
	}

	// 主讲人id
	//stream, _ := dao.Stream.QueryStream(v.StreamId)
	//if stream == nil {
	//	return nil, fmt.Errorf("找不到主讲人")
	//}

	device := &stack.Device{DeviceModel: model}
	_, err := device.StartBroadcast(v.StreamId, v.DeviceID, v.ChannelID, r.Context())
	return nil, err
}

func (api *ApiServer) OnTalk(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceId := vars["device"]
	channelId := vars["channel"]

	_, online := stack.OnlineDeviceManager.Find(deviceId)
	if !online {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, "设备离线")
		return
	}

	model, err := dao.Device.QueryDevice(deviceId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, "设备不存在")
		return
	}

	// 目前只实现livegbs的一对一的对讲, stream id就是通道的广播id
	streamid := common.GenerateStreamID(common.InviteTypeBroadcast, deviceId, channelId, "", "")
	device := &stack.Device{DeviceModel: model}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	sinkModel, err := device.StartBroadcast(streamid, deviceId, channelId, ctx)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = common.HttpResponseJson(w, "广播失败")
		return
	}

	err = common.WSForwardTo(r.URL.Path, w, r)
	if err != nil {
		log.Sugar.Errorf("广播失败 err: %s", err.Error())
	}

	log.Sugar.Infof("广播结束 device: %s/%s", deviceId, channelId)

	// 对讲结束, 关闭sink
	sink := &stack.Sink{SinkModel: sinkModel}
	sink.Close(true, true)
}
