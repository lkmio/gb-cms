package main

import "sync"

var (
	snValue   int
	SNManager snManager
)

type EventCb func(data interface{})

func init() {
	SNManager.events = make(map[int]EventCb, 1024)
}

func GetSN() int {
	for snValue < 0xFFFFFF {
		snValue = (snValue + 1) % 0xFFFFFF
		if SNManager.FindEvent(snValue) == nil {
			return snValue
		}
	}

	return 0
}

type snManager struct {
	events map[int]EventCb
	lock   sync.RWMutex
}

func (s *snManager) AddEvent(sn int, cb EventCb) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.events[sn] = cb
}

func (s *snManager) FindEvent(sn int) EventCb {
	s.lock.RLock()
	cb, ok := s.events[sn]
	s.lock.RUnlock()
	if ok {
		return cb
	}

	return nil
}

func (s *snManager) RemoveEvent(sn int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.events, sn)
}
