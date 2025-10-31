package stack

import (
	"gb-cms/dao"
	"time"
)

func RefreshCatalogScheduleTask() {
	now := time.Now()
	devices, _ := dao.Device.QueryRefreshCatalogExpiredDevices(now)
	// 发起查询目录请求
	for _, device := range devices {
		d := &Device{device}
		go func() {
			_, _ = d.QueryCatalog(30)
		}()
	}
}

func RefreshSubscribeScheduleTask() {
	now := time.Now()
	dialogs, _ := dao.Dialog.QueryExpiredDialogs(now)
	for _, dialog := range dialogs {
		go func(dialog *dao.SipDialogModel) {
			device, _ := dao.Device.QueryDevice(dialog.DeviceID)
			if device == nil {
				// 被上级订阅, 由上级刷新, 过期删除
				if dialog.RefreshTime.Before(now) {
					_ = dao.Dialog.DeleteDialogByCallID(dialog.CallID)
				}
				return
			}

			d := &Device{device}
			if dao.SipDialogTypeSubscribeCatalog == dialog.Type {
				d.RefreshSubscribeCatalog()
			} else if dao.SipDialogTypeSubscribePosition == dialog.Type {
				d.RefreshSubscribePosition()
			} else if dao.SipDialogTypeSubscribeAlarm == dialog.Type {
				d.RefreshSubscribeAlarm()
			}
		}(dialog)
	}
}

func AddScheduledTask(interval time.Duration, firstRun bool, task func()) {
	ticker := time.NewTicker(interval)
	if firstRun {
		go task()
	}

	for {
		select {
		case <-ticker.C:
			go task()
		}
	}
}
