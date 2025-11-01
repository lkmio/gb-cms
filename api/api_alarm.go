package api

import (
	"gb-cms/common"
	"gb-cms/dao"
	"net/http"
)

func (api *ApiServer) OnAlarmList(q *QueryDeviceChannel, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	if q.Limit < 1 {
		q.Limit = 10
	}

	conditions := make(map[string]interface{}, 0)

	// 报警查询参数
	if q.Keyword != "" {
		conditions["q"] = q.Keyword
	}

	if q.StartTime != "" {
		conditions["starttime"] = q.StartTime
	}

	if q.EndTime != "" {
		conditions["endtime"] = q.EndTime
	}

	if q.Priority > 0 {
		conditions["alarm_priority"] = q.Priority
	}

	if q.Method != "" {
		conditions["alarm_method"] = q.Method
	}

	alarms, count, err := dao.Alarm.QueryAlarmList((q.Start/q.Limit)+1, q.Limit, conditions)

	if err != nil {
		return nil, err
	}

	v := struct {
		AlarmCount          int
		AlarmList           []*dao.AlarmModel
		AlarmPublishToRedis bool
		AlarmReserveDays    int
	}{
		AlarmCount:          count,
		AlarmList:           alarms,
		AlarmPublishToRedis: true,
		AlarmReserveDays:    common.Config.AlarmReserveDays,
	}

	return &v, nil
}

func (api *ApiServer) OnAlarmRemove(params *SetEnable, _ http.ResponseWriter, _ *http.Request) (interface{}, error) {
	// 删除报警
	if err := dao.Alarm.Delete(params.ID); err != nil {
		return nil, err
	}

	return "OK", nil
}

func (api *ApiServer) OnAlarmClear(_ *Empty, _ http.ResponseWriter, req *http.Request) (interface{}, error) {
	// 清空报警
	if err := dao.Alarm.Clear(); err != nil {
		return nil, err
	}

	return "OK", nil
}
