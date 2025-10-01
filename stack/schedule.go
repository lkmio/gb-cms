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
