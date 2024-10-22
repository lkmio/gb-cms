package main

import "github.com/ghettovoice/gosip/sip"

type GBSubscribe struct {
	PositionDialog sip.Request
	CatalogDialog  sip.Request
	AlarmDialog    sip.Request
}

func RefreshSubscribe(expires int) {

}
