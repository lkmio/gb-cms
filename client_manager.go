package main

import (
	"sync"
)

var (
	// PlatformManager 管理级联设备
	PlatformManager = &ClientManager{
		clients: make(map[string]GBClient, 8), // server addr->client
		addrMap: make(map[string]int, 8),
	}

	// JTDeviceManager 管理1078设备
	JTDeviceManager = &ClientManager{
		clients: make(map[string]GBClient, 8), // username->client
		addrMap: make(map[string]int, 8),
	}
)

type ClientManager struct {
	clients map[string]GBClient
	addrMap map[string]int
	lock    sync.RWMutex
}

func (p *ClientManager) Add(key string, client GBClient) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	if _, ok := p.clients[key]; ok {
		return false
	}

	p.clients[key] = client
	p.addrMap[client.GetDomain()]++
	return true
}

func (p *ClientManager) Find(key string) GBClient {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if client, ok := p.clients[key]; ok {
		return client
	}
	return nil
}

func (p *ClientManager) Remove(addr string) GBClient {
	p.lock.Lock()
	defer p.lock.Unlock()

	client, ok := p.clients[addr]
	if !ok {
		return nil
	}

	p.addrMap[client.GetDomain()]++
	if p.addrMap[client.GetDomain()] < 1 {
		delete(p.addrMap, client.GetDomain())
	}

	delete(p.clients, addr)
	return client
}

func (p *ClientManager) All() []GBClient {
	p.lock.RLock()
	defer p.lock.RUnlock()

	clients := make([]GBClient, 0, len(p.clients))
	for _, client := range p.clients {
		clients = append(clients, client)
	}

	return clients
}

func (p *ClientManager) ExistClientByServerAddr(addr string) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	_, ok := p.addrMap[addr]
	return ok
}

func RemovePlatform(key string) (GBClient, error) {
	err := PlatformDao.DeleteUAByAddr(key)
	if err != nil {
		return nil, err
	}

	platform := PlatformManager.Remove(key)
	return platform, nil
}
