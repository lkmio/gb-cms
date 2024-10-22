package main

import "sync"

type BroadcastRoom struct {
	members map[string]*BroadcastSession
	lock    sync.RWMutex
}

func (r *BroadcastRoom) Add(session *BroadcastSession) bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	if _, ok := r.members[session.SourceID]; ok {
		return false
	}

	r.members[session.SourceID] = session
	return true
}

func (r *BroadcastRoom) Remove(sourceId string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.members[sourceId]
	if !ok {
		return
	}

	delete(r.members, sourceId)
}

func (r *BroadcastRoom) Exist(sourceId string) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()

	_, ok := r.members[sourceId]
	return ok
}

func (r *BroadcastRoom) Find(sourceId string) *BroadcastSession {
	r.lock.RLock()
	defer r.lock.RUnlock()

	session, _ := r.members[sourceId]
	return session
}

func (r *BroadcastRoom) PopAll() []*BroadcastSession {
	r.lock.Lock()
	defer r.lock.Unlock()

	var members []*BroadcastSession
	for _, session := range r.members {
		members = append(members, session)
	}

	return members
}

func (r *BroadcastRoom) DispatchRtpPacket(data []byte) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for _, session := range r.members {
		session.Write(data)
	}
}
