package api

import (
	"container/list"
	"context"
	"sync"
	"time"
)

const (
	SnapshotQueueMaxSize = 128
)

var (
	DefaultSnapshotManager = &SnapshotManager{
		snapshots: make(map[string][]func(), SnapshotQueueMaxSize*1.5),
		keys:      list.New(),
	}
)

type SnapshotManager struct {
	lock      sync.RWMutex
	snapshots map[string][]func()
	keys      *list.List
}

func (s *SnapshotManager) Get(id string, timeout time.Duration) bool {
	s.lock.Lock()
	if _, ok := s.snapshots[id]; ok {
		s.lock.Unlock()
		return true
	}

	background, cancel := context.WithCancel(context.Background())
	s.snapshots[id] = append(s.snapshots[id], cancel)
	// 超时未获取到截图, 则认为不存在
	after := time.After(timeout)

	s.lock.Unlock()
	select {
	case <-after:
		return false
	case <-background.Done():
		return true
	}
}

func (s *SnapshotManager) Put(id string, ok bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// 通知获取截图的goroutine, 截图已准备好
	for _, cancel := range s.snapshots[id] {
		cancel()
	}

	// 超出最大队列长度, 则删除最早的截图
	for s.keys.Len() > SnapshotQueueMaxSize {
		oldElement := s.keys.Front()
		s.keys.Remove(oldElement)
		delete(s.snapshots, oldElement.Value.(string))
	}
}
