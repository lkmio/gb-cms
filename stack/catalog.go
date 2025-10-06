package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"github.com/ghettovoice/gosip/sip"
)

const (
	EventCatalog = "catalog"
)

// SubscribeCatalog 域间目录订阅
func (d *Device) SubscribeCatalog() error {
	builder := d.NewRequestBuilder(sip.SUBSCRIBE, common.Config.SipID, common.Config.SipContactAddr, d.DeviceID)
	body := fmt.Sprintf(CatalogFormat, GetSN(), d.DeviceID)

	expiresHeader := sip.Expires(common.Config.SubscribeExpires)
	builder.SetExpires(&expiresHeader)
	builder.SetContentType(&XmlMessageType)
	builder.SetContact(GlobalContactAddress)
	builder.SetBody(body)

	request, err := builder.Build()
	if err != nil {
		return err
	}

	err = SendSubscribeMessage(d.DeviceID, request, dao.SipDialogTypeSubscribeCatalog, EventCatalog)
	if err != nil {
		log.Sugar.Errorf("订阅目录失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}

	return err
}

func (d *Device) UnsubscribeCatalog() {
	body := fmt.Sprintf(CatalogFormat, GetSN(), d.DeviceID)
	err := Unsubscribe(d.DeviceID, dao.SipDialogTypeSubscribeCatalog, EventCatalog, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("取消订阅目录失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}

func (d *Device) RefreshSubscribeCatalog() {
	body := fmt.Sprintf(CatalogFormat, GetSN(), d.DeviceID)
	err := RefreshSubscribe(d.DeviceID, dao.SipDialogTypeSubscribeCatalog, EventCatalog, common.Config.SubscribeExpires, []byte(body), d.RemoteIP, d.RemotePort)
	if err != nil {
		log.Sugar.Errorf("刷新目录订阅失败 err: %s deviceID: %s", err.Error(), d.DeviceID)
	}
}
