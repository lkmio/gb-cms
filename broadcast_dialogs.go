package main

import "sync"

// BroadcastDialogs 临时保存广播会话
var BroadcastDialogs = &broadcastDialogs{
	dialogs: make(map[string]*Sink),
}

type broadcastDialogs struct {
	lock    sync.RWMutex
	dialogs map[string]*Sink
}

func (b *broadcastDialogs) Add(id string, dialog *Sink) (old *Sink, ok bool) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if old, ok = b.dialogs[id]; ok {
		return old, false
	}

	b.dialogs[id] = dialog
	return nil, true
}

func (b *broadcastDialogs) Find(id string) *Sink {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.dialogs[id]
}

func (b *broadcastDialogs) Remove(id string) *Sink {
	b.lock.Lock()
	defer b.lock.Unlock()
	dialog := b.dialogs[id]
	delete(b.dialogs, id)
	return dialog
}
