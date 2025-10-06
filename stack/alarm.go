package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
	"time"
)

const (
	AlarmFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Query>\r\n" +
		"<CmdType>Alarm</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<StartAlarmPriority>1</StartAlarmPriority>\r\n" + //  <!-- 报警起始级别(可选),0为全部,1为一级警情,2为二级警情,3为三级警情,4为四级警情-->
		"<EndAlarmPriority>4</EndAlarmPriority>\r\n" + //  <!-- 报警终止级别(可选),0为全部,1为一级警情,2为二级警情,3为三级警情,4为四级警情-->
		"<AlarmMethod>0</AlarmMethod>\r\n" + // <!-- 报警方式条件(可选),取值0为全部,1为电话报警,2为设备报警,3为短信报警,4为GPS报警,5为视频报警,6为设备故障报警,7其他报警;可以为直接组合如12为电话报警或设备报警-->
		"<StartTime>%s</StartTime>\r\n" +
		"<EndTime>%s</EndTime>\r\n" +
		"</Query>"

	AlarmResponseFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Response>\r\n" +
		"<CmdType>Alarm</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<Result>%s</Result>\r\n" +
		"</Response>"
)

type AlarmNotify struct {
	BaseMessage
	AlarmPriority    string `xml:"AlarmPriority"`    // <!-- 报警级别(必选),1为一级警情,2为二级警情,3为三级警情,4为四级警情-->
	AlarmMethod      string `xml:"AlarmMethod"`      // <!-- 报警方式(必选),取值1为电话报警,2为设备报警,3为短信报警,4为GPS报警,5为视频报警,6为设备故障报警,7其他报警-->
	AlarmTime        string `xml:"AlarmTime"`        // <!--报警时间(必选)-->
	AlarmDescription string `xml:"AlarmDescription"` // <!--报警内容描述(可选)-->
	Longitude        string `xml:"Longitude"`        // <!-- 经度(可选)-->
	Latitude         string `xml:"Latitude"`         // <!-- 纬度(可选)-->
	Info             *struct {
		// <!-- 报警类型。报警方式为2时,不携带AlarmType为默认的报警设备报警,携带AlarmType取值及对应报警类型如下:
		// 1-视频丢失报警;2-设备防拆报警;3-存储设备磁盘满报警;4-设备高温报警;5-设备低温报警。报警方式为5时,取值如下:
		// 1-人工视频报警;2-运动目标检测报警;3-遗留物检测报警;4-物体移除检测报警;5-绊线检测报警;6-入侵检测报警;7-逆行检测报警;8-徘徊检测报警;9-流量统计报警;
		// 10-密度检测报警;11-视频异常检测报警;12-快速移动报警。报警方式为6时,取值如下:1-存储设备磁盘故障报警;2-存储设备风扇故障报警。-->
		AlarmType *int `xml:"AlarmType"`

		// <!—报警类型扩展参数。在入侵检测报警时可携带<EventType>事件类型</EventType>,事件类型取值:1-进入区域;2-离开区域。-->
		AlarmTypeParam *struct {
			EventType *int `xml:"EventType"`
		}
	} `xml:"Info"`
}

func (d *Device) SubscribeAlarm() error {
	now := time.Now()
	end := now.Add(time.Duration(common.Config.SubscribeExpires) * time.Second)
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, common.Config.SipID, common.Config.SipContactAddr, d.DeviceID)
	body := fmt.Sprintf(AlarmFormat, GetSN(), d.DeviceID, now.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))

	expiresHeader := sip.Expires(common.Config.SubscribeExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	err = SendSubscribeMessage(d.DeviceID, request, dao.SipDialogTypeSubscribeAlarm, EventPresence)
	if err != nil {
		log.Sugar.Errorf("订阅报警失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}

	return err
}

func (d *Device) UnsubscribeAlarm() {
	body := fmt.Sprintf(MobilePositionMessageFormatUnsubscribe, GetSN(), d.DeviceID)
	err := Unsubscribe(d.DeviceID, dao.SipDialogTypeSubscribeAlarm, EventPresence, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("取消订阅报警失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}

func (d *Device) RefreshSubscribeAlarm() {
	now := time.Now()
	end := now.Add(time.Duration(common.Config.SubscribeExpires) * time.Second)
	body := fmt.Sprintf(AlarmFormat, GetSN(), d.DeviceID, now.Format("2006-01-02T15:04:05"), end.Format("2006-01-02T15:04:05"))
	err := RefreshSubscribe(d.DeviceID, dao.SipDialogTypeSubscribeAlarm, EventPresence, common.Config.SubscribeExpires, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("刷新报警订阅失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}

// SendAlarmNotificationResponseCmd 设备主动上报的报警信息，需要回复确认
func (d *Device) SendAlarmNotificationResponseCmd(sn int, id string) {
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, common.Config.SipID, common.Config.SipContactAddr, d.DeviceID)
	builder.SetBody(fmt.Sprintf(AlarmResponseFormat, sn, id, "OK"))
	request, err := builder.Build()
	if err != nil {
		return
	}

	common.SipStack.SendRequest(request)
}
