package dao

import (
	"strings"
	"sync"
)

var BlacklistManager = &blacklistManager{
	uaList: make(map[string]string),
	ipList: make(map[string]string),
}

type blacklistManager struct {
	lock   sync.RWMutex
	uaList map[string]string
	ipList map[string]string
}

func (b *blacklistManager) QueryIP(ip string) (*BlacklistModel, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	if _, ok := b.ipList[ip]; !ok {
		return nil, nil
	}

	return &BlacklistModel{
		Key:  ip,
		Rule: "ip",
	}, nil
}

func (b *blacklistManager) QueryUA(ua string) (*BlacklistModel, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	if _, ok := b.uaList[ua]; !ok {
		return nil, nil
	}

	return &BlacklistModel{
		Key:  ua,
		Rule: "ua",
	}, nil
}

func (b *blacklistManager) SaveIP(ip string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.ipList[ip] = ip
	return nil
}

func (b *blacklistManager) SaveUA(ua string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.uaList[ua] = ua
	return nil
}

func (b *blacklistManager) DeleteIP(ip string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.ipList, ip)
	return nil
}

func (b *blacklistManager) DeleteUA(ua string) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.uaList, ua)
	return nil
}

func (b *blacklistManager) ToStrings() (string, string) {
	b.lock.RLock()
	defer b.lock.RUnlock()
	var ipList []string
	for ip := range b.ipList {
		ipList = append(ipList, ip)
	}

	var uaList []string
	for ua := range b.uaList {
		uaList = append(uaList, ua)
	}

	return strings.Join(ipList, ","), strings.Join(uaList, ",")
}

func (b *blacklistManager) Clear() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.ipList = make(map[string]string)
	b.uaList = make(map[string]string)
}
