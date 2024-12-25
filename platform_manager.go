package main

import "sync"

var (
	PlatformManager *platformManager
)

func init() {
	PlatformManager = &platformManager{
		platforms: make(map[string]interface{}, 8),
		addrMap:   make(map[string]interface{}, 8),
	}
}

type platformManager struct {
	platforms map[string]interface{} //上级id->平台
	addrMap   map[string]interface{} //上级地址->平台
	lock      sync.RWMutex
}

func (p *platformManager) AddPlatform(platform *GBPlatform) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	// 以上级平台ID作为主键
	if _, ok := p.addrMap[platform.sipClient.SeverID]; ok {
		return false
	}

	p.platforms[platform.sipClient.SeverID] = platform
	p.addrMap[platform.sipClient.Domain] = platform
	return true
}

func (p *platformManager) ExistPlatform(id string) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	_, ok := p.platforms[id]
	return ok
}

func (p *platformManager) ExistPlatformWithServerAddr(addr string) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	_, ok := p.addrMap[addr]
	return ok
}

func (p *platformManager) FindPlatform(id string) *GBPlatform {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if platform, ok := p.platforms[id]; ok {
		return platform.(*GBPlatform)
	}
	return nil
}

func (p *platformManager) RemovePlatform(id string) *GBPlatform {
	p.lock.Lock()
	defer p.lock.Unlock()

	platform, ok := p.platforms[id]
	if !ok {
		return nil
	}

	delete(p.platforms, id)
	delete(p.addrMap, platform.(*GBPlatform).sipClient.Domain)
	return platform.(*GBPlatform)
}

func (p *platformManager) FindPlatformWithServerAddr(addr string) *GBPlatform {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if platform, ok := p.addrMap[addr]; ok {
		return platform.(*GBPlatform)
	}
	return nil
}

func (p *platformManager) Platforms() []*GBPlatform {
	p.lock.RLock()
	defer p.lock.RUnlock()

	var platforms []*GBPlatform
	for _, platform := range p.platforms {
		platforms = append(platforms, platform.(*GBPlatform))
	}
	return platforms
}
