package dao

import (
	"gb-cms/common"
	"gorm.io/gorm"
	"time"
)

type AlarmModel struct {
	GBModel
	DeviceID          string
	DeviceName        string // 设备名称, 方便模糊查询
	ChannelID         string
	ChannelName       string // 通道名称, 方便模糊查询
	AlarmPriority     int
	AlarmPriorityName string
	AlarmMethod       int
	AlarmMethodName   string
	Time              string
	Description       *string
	Longitude         *float64
	Latitude          *float64
	AlarmType         *int
	AlarmTypeName     string
	EventType         *int
}

func (a *AlarmModel) TableName() string {
	return "lkm_alarm"
}

type daoAlarm struct {
}

func (d *daoAlarm) Save(alarm *AlarmModel) error {
	if 1 == alarm.AlarmPriority {
		alarm.AlarmPriorityName = "一级警情"
	} else if 2 == alarm.AlarmPriority {
		alarm.AlarmPriorityName = "二级警情"
	} else if 3 == alarm.AlarmPriority {
		alarm.AlarmPriorityName = "三级警情"
	} else if 4 == alarm.AlarmPriority {
		alarm.AlarmPriorityName = "四级警情"
	}

	if 1 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "电话报警"
	} else if 2 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "设备报警"
	} else if 3 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "短信报警"
	} else if 4 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "GPS报警"
	} else if 5 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "视频报警"
	} else if 6 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "设备故障报警"
	} else if 7 == alarm.AlarmMethod {
		alarm.AlarmMethodName = "其他报警"
	}

	// <!-- 报警类型。报警方式为2时,不携带AlarmType为默认的报警设备报警,携带AlarmType取值及对应报警类型如下:
	// 1-视频丢失报警;2-设备防拆报警;3-存储设备磁盘满报警;4-设备高温报警;5-设备低温报警。报警方式为5时,取值如下:
	// 1-人工视频报警;2-运动目标检测报警;3-遗留物检测报警;4-物体移除检测报警;5-绊线检测报警;6-入侵检测报警;7-逆行检测报警;8-徘徊检测报警;9-流量统计报警;
	// 10-密度检测报警;11-视频异常检测报警;12-快速移动报警。报警方式为6时,取值如下:1-存储设备磁盘故障报警;2-存储设备风扇故障报警。-->
	if 2 == alarm.AlarmMethod {
		if alarm.AlarmType == nil {
			alarm.AlarmTypeName = "设备报警"
		} else if 1 == *alarm.AlarmType {
			alarm.AlarmTypeName = "视频丢失报警"
		} else if 2 == *alarm.AlarmType {
			alarm.AlarmTypeName = "设备防拆报警"
		} else if 3 == *alarm.AlarmType {
			alarm.AlarmTypeName = "存储设备磁盘满报警"
		} else if 4 == *alarm.AlarmType {
			alarm.AlarmTypeName = "设备高温报警"
		} else if 5 == *alarm.AlarmType {
			alarm.AlarmTypeName = "设备低温报警"
		}
	} else if 5 == alarm.AlarmMethod && alarm.AlarmType != nil {
		if 1 == *alarm.AlarmType {
			alarm.AlarmTypeName = "人工视频报警"
		} else if 2 == *alarm.AlarmType {
			alarm.AlarmTypeName = "运动目标检测报警"
		} else if 3 == *alarm.AlarmType {
			alarm.AlarmTypeName = "遗留物检测报警"
		} else if 4 == *alarm.AlarmType {
			alarm.AlarmTypeName = "物体移除检测报警"
		} else if 5 == *alarm.AlarmType {
			alarm.AlarmTypeName = "绊线检测报警"
		} else if 6 == *alarm.AlarmType {
			alarm.AlarmTypeName = "入侵检测报警"
		} else if 7 == *alarm.AlarmType {
			alarm.AlarmTypeName = "逆行检测报警"
		} else if 8 == *alarm.AlarmType {
			alarm.AlarmTypeName = "徘徊检测报警"
		} else if 9 == *alarm.AlarmType {
			alarm.AlarmTypeName = "流量统计报警"
		} else if 10 == *alarm.AlarmType {
			alarm.AlarmTypeName = "密度检测报警"
		} else if 11 == *alarm.AlarmType {
			alarm.AlarmTypeName = "视频异常检测报警"
		} else if 12 == *alarm.AlarmType {
			alarm.AlarmTypeName = "快速移动报警"
		}
	} else if 6 == alarm.AlarmMethod && alarm.AlarmType != nil {
		if 1 == *alarm.AlarmType {
			alarm.AlarmTypeName = "存储设备磁盘故障报警"
		} else if 2 == *alarm.AlarmType {
			alarm.AlarmTypeName = "存储设备风扇故障报警"
		}
	}

	deviceName, _ := Device.QueryDeviceName(alarm.DeviceID)
	channelName, _ := Channel.QueryChannelName(alarm.DeviceID, alarm.ChannelID)
	alarm.DeviceName = deviceName
	alarm.ChannelName = channelName
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Save(alarm).Error
	})
}

// QueryAlarmList 分页查询报警列表
func (d *daoAlarm) QueryAlarmList(page int, size int, conditions map[string]interface{}) ([]*AlarmModel, int, error) {
	tx := db.Limit(size).Offset((page - 1) * size)

	if v, ok := conditions["order"]; ok && v != "desc" {
		tx.Order("id asc")
	} else {
		tx.Order("id desc")
	}

	if v, ok := conditions["q"]; ok && v != "" {
		tx.Where("description like ? or device_id like ? or channel_id like ? or device_name like ? or channel_name like ? or alarm_type_name like ? or alarm_method_name like ? or alarm_priority_name like ?", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%", "%"+v.(string)+"%")
	}

	if v, ok := conditions["starttime"]; ok && v != "" {
		tx.Where("created_at >= ?", common.ParseGBTime(v.(string)))
	}

	if v, ok := conditions["endtime"]; ok && v != "" {
		tx.Where("created_at <= ?", common.ParseGBTime(v.(string)))
	}

	if v, ok := conditions["alarm_priority"]; ok && v.(int) > 0 {
		tx.Where("alarm_priority = ?", v.(int))
	}

	if v, ok := conditions["alarm_method"]; ok && v.(int) > 0 {
		tx.Where("alarm_method = ?", v.(int))
	}

	var alarms []*AlarmModel
	if tx := tx.Find(&alarms); tx.Error != nil {
		return nil, 0, tx.Error
	}

	var count int64
	tx.Count(&count)

	return alarms, int(count), nil
}

// DeleteAlarm 删除报警
func (d *daoAlarm) DeleteAlarm(id int) error {
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Delete(&AlarmModel{}, id).Unscoped().Error
	})
}

func (d *daoAlarm) ClearAlarm() error {
	// 清空报警
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM lkm_alarm;").Error
	})
}

func (d *daoAlarm) DeleteExpired(time time.Time) error {
	// 删除过期的报警记录
	return DBTransaction(func(tx *gorm.DB) error {
		return tx.Where("created_at < ?", time).Delete(&AlarmModel{}).Unscoped().Error
	})
}
