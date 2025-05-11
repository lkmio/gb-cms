package main

import (
	"fmt"
	"sync"
)

var (
	PlatformManager = &platformManager{
		addrMap: make(map[string]*GBPlatform, 8),
	}
)

type platformManager struct {
	addrMap map[string]*GBPlatform //上级地址->平台
	lock    sync.RWMutex
}

func (p *platformManager) Add(platform *GBPlatform) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	if _, ok := p.addrMap[platform.sipClient.ServerAddr]; ok {
		return false
	}

	p.addrMap[platform.sipClient.ServerAddr] = platform
	return true
}

func (p *platformManager) Find(addr string) *GBPlatform {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if platform, ok := p.addrMap[addr]; ok {
		return platform
	}
	return nil
}

func (p *platformManager) Remove(addr string) *GBPlatform {
	p.lock.Lock()
	defer p.lock.Unlock()

	platform, ok := p.addrMap[addr]
	if !ok {
		return nil
	}

	delete(p.addrMap, addr)
	return platform
}
func (p *platformManager) Platforms() []*GBPlatform {
	p.lock.RLock()
	defer p.lock.RUnlock()

	platforms := make([]*GBPlatform, 0, len(p.addrMap))
	for _, platform := range p.addrMap {
		platforms = append(platforms, platform)
	}

	return platforms
}

func AddPlatform(platform *GBPlatform) error {
	ok := PlatformManager.Add(platform)
	if !ok {
		return fmt.Errorf("平台添加失败, 地址冲突. addr: %s", platform.sipClient.ServerAddr)
	}

	if DB != nil {
		err := DB.SavePlatform(&platform.SIPUAParams)
		if err != nil {
			PlatformManager.Remove(platform.sipClient.ServerAddr)
			return fmt.Errorf("平台保存到数据库失败, err: %s", err.Error())
		}
	}

	return nil
}

func RemovePlatform(addr string) (*GBPlatform, error) {
	if DB != nil {
		err := DB.DeletePlatform(addr)
		if err != nil {
			return nil, err
		}
	}

	platform := PlatformManager.Remove(addr)
	return platform, nil
}

func LoadPlatforms() []*SIPUAParams {
	platforms := PlatformManager.Platforms()
	params := make([]*SIPUAParams, 0, len(platforms))
	for _, platform := range platforms {
		params = append(params, &platform.SIPUAParams)
	}

	return params
}

func QueryPlatform(add string) *GBPlatform {
	return PlatformManager.Find(add)
}

func UpdatePlatformStatus(addr string, status OnlineStatus) error {
	platform := PlatformManager.Find(addr)
	if platform == nil {
		return fmt.Errorf("平台不存在. addr: %s", addr)
	}

	//old := platform.Device.Status
	platform.Device.Status = status

	if DB != nil {
		err := DB.UpdatePlatformStatus(addr, status)
		// platform.Device.Status = old
		if err != nil {
			return err
		}
	}

	return nil
}
