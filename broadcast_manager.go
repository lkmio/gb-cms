package main

import "sync"

//var BroadcastManager = &broadcastManager{
//	streams: make(map[StreamID]*Sink),
//	callIds: make(map[string]*Sink),
//}

type broadcastManager struct {
	streams map[StreamID]*Sink // device stream id ->sink
	callIds map[string]*Sink   // invite call id->sink
	lock    sync.RWMutex
}

func (b *broadcastManager) Add(id StreamID, sink *Sink) (old *Sink, ok bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
	old, ok = b.streams[id]
	if ok {
		return old, false
	}
	b.streams[id] = sink
	return nil, true
}

func (b *broadcastManager) AddWithCallId(id string, sink *Sink) bool {
	b.lock.Lock()
	defer b.lock.Unlock()
	if _, ok := b.callIds[id]; ok {
		return false
	}
	b.callIds[id] = sink
	return true
}

func (b *broadcastManager) Find(id StreamID) *Sink {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.streams[id]
}

func (b *broadcastManager) FindWithCallId(id string) *Sink {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.callIds[id]
}

func (b *broadcastManager) Remove(id StreamID) *Sink {
	b.lock.Lock()
	defer b.lock.Unlock()
	sink, ok := b.streams[id]
	if !ok {
		return nil
	}

	if sink.Dialog != nil {
		callID, _ := sink.Dialog.CallID()
		delete(b.callIds, callID.String())
	}

	delete(b.streams, id)
	return sink
}

func (b *broadcastManager) RemoveWithCallId(id string) *Sink {
	b.lock.Lock()
	defer b.lock.Unlock()
	sink, ok := b.callIds[id]
	if !ok {
		return nil
	}

	delete(b.callIds, id)
	delete(b.streams, sink.Stream)
	return sink
}
