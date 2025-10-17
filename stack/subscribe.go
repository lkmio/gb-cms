package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"github.com/ghettovoice/gosip/sip"
	"time"
)

// SubscribeEvent 发起订阅
func (d *Device) SubscribeEvent() {
	if d.PositionSubscribe {
		// 先取消订阅之前的,再重新发起订阅
		dialogs, _ := dao.Dialog.QueryDialogsByType(d.DeviceID, dao.SipDialogTypeSubscribePosition)
		if len(dialogs) > 0 {
			d.UnsubscribePosition()
		}
		_ = d.SubscribePosition()
	}

	if d.CatalogSubscribe {
		// 先取消订阅之前的,再重新发起订阅
		dialogs, _ := dao.Dialog.QueryDialogsByType(d.DeviceID, dao.SipDialogTypeSubscribeCatalog)
		if len(dialogs) > 0 {
			d.UnsubscribeCatalog()
		}
		_ = d.SubscribeCatalog()
	}

	if d.AlarmSubscribe {
		// 先取消订阅之前的,再重新发起订阅
		dialogs, _ := dao.Dialog.QueryDialogsByType(d.DeviceID, dao.SipDialogTypeSubscribeAlarm)
		if len(dialogs) > 0 {
			d.UnsubscribeAlarm()
		}
		_ = d.SubscribeAlarm()
	}
}

// SendSubscribeMessage 通用发送订阅消息
func SendSubscribeMessage(deviceId string, request sip.Request, t int, event Event) error {
	request.AppendHeader(&event)

	transaction := common.SipStack.SendRequest(request)
	response := <-transaction.Responses()
	if response == nil {
		return fmt.Errorf("no response")
	} else if response.StatusCode() != 200 {
		return fmt.Errorf("error response code: %d", response.StatusCode())
	}

	// 保存dialog到数据库
	dialog := CreateDialogRequestFromAnswer(response, false, request.Source())
	callid, _ := dialog.CallID()
	model := &dao.SipDialogModel{
		DeviceID:    deviceId,
		CallID:      callid.Value(),
		Dialog:      &common.RequestWrapper{Request: dialog},
		Type:        t,
		RefreshTime: time.Now().Add(time.Duration(common.Config.SubscribeExpires-60) * time.Second), // 刷新订阅时间, -60秒预留计时器出发间隔, 确保订阅在过期前刷新
	}

	return dao.Dialog.Save(model)
}

// Unsubscribe 通用取消订阅消息
func Unsubscribe(deviceId string, t int, event Event, body []byte, remoteIP string, remotePort int) error {
	model, err := dao.Dialog.DeleteDialogsByType(deviceId, t)
	if err != nil {
		return err
	}

	request := CreateRequestFromDialog(model.Dialog.Request, sip.SUBSCRIBE, remoteIP, remotePort)

	// 添加事件头
	expiresHeader := sip.Expires(0)

	common.SetHeader(request, &event)
	common.SetHeader(request, &expiresHeader)
	common.SetHeader(request, GlobalContactAddress.AsContactHeader())
	common.SetHeader(request, &XmlMessageType)

	if body != nil {
		request.SetBody(string(body), true)
	}

	common.SipStack.SendRequest(request)
	return nil
}

func RefreshSubscribe(deviceId string, t int, event Event, expires int, body []byte, remoteIP string, remotePort int) error {
	dialogs, _ := dao.Dialog.QueryDialogsByType(deviceId, t)
	if len(dialogs) == 0 {
		return fmt.Errorf("no dialog")
	}

	request := CreateRequestFromDialog(dialogs[0].Dialog.Request, sip.SUBSCRIBE, remoteIP, remotePort)

	expiresHeader := sip.Expires(expires)

	common.SetHeader(request, &event)
	common.SetHeader(request, &expiresHeader)
	common.SetHeader(request, GlobalContactAddress.AsContactHeader())
	common.SetHeader(request, &XmlMessageType)

	if body != nil {
		request.SetBody(string(body), true)
	}

	transaction := common.SipStack.SendRequest(request)
	response := <-transaction.Responses()
	if response == nil {
		return fmt.Errorf("no response")
	} else if response.StatusCode() != 200 {
		return fmt.Errorf("error response code: %d", response.StatusCode())
	} else {
		// 刷新订阅时间, -60秒预留计时器触发间隔, 确保订阅在过期前刷新
		dialogs[0].RefreshTime = time.Now().Add(time.Duration(common.Config.SubscribeExpires-60) * time.Second)
		err := dao.Dialog.Save(dialogs[0])
		if err != nil {
			return err
		}
	}

	return nil
}
