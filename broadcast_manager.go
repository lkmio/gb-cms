package main

import (
	"sync"
)

var (
	BroadcastManager *broadcastManager
)

func init() {
	BroadcastManager = &broadcastManager{
		rooms:    make(map[string]*BroadcastRoom, 12),
		sessions: make(map[string]*BroadcastSession, 12),
		callIds:  make(map[string]*BroadcastSession, 12),
	}
}

type broadcastManager struct {
	rooms    map[string]*BroadcastRoom    //主讲人关联房间
	sessions map[string]*BroadcastSession //sessionId关联广播会话
	callIds  map[string]*BroadcastSession //callId关联广播会话
	lock     sync.RWMutex
}

func FindBroadcastSessionWithSourceID(user string) *BroadcastSession {
	roomId := user[:10]
	room := BroadcastManager.FindRoom(roomId)
	if room != nil {
		return room.Find(user)
	}

	return nil
}

func (b *broadcastManager) CreateRoom(id string) *BroadcastRoom {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.rooms[id]; ok {
		panic("system error")
	}

	room := &BroadcastRoom{
		members: make(map[string]*BroadcastSession, 12),
	}
	b.rooms[id] = room
	return room
}

func (b *broadcastManager) FindRoom(id string) *BroadcastRoom {
	b.lock.RLock()
	defer b.lock.RUnlock()

	session, ok := b.rooms[id]
	if !ok {
		return nil
	}

	return session
}

func (b *broadcastManager) RemoveRoom(roomId string) []*BroadcastSession {
	b.lock.Lock()
	defer b.lock.Unlock()

	room, ok := b.rooms[roomId]
	if !ok {
		return nil
	}

	delete(b.rooms, roomId)
	return room.PopAll()
}

func (b *broadcastManager) Remove(sessionId string) *BroadcastSession {
	b.lock.Lock()
	defer b.lock.Unlock()

	session, ok := b.sessions[sessionId]
	if !ok {
		return nil
	}

	b.RemoveSession(session)
	return session
}

func (b *broadcastManager) RemoveSession(session *BroadcastSession) {
	delete(b.sessions, session.Id())

	if session.ByeRequest != nil {
		id, _ := session.ByeRequest.CallID()
		delete(b.callIds, id.Value())
	}

	if room, ok := b.rooms[session.RoomId]; ok {
		room.Remove(session.SourceID)
	}
}

func (b *broadcastManager) RemoveWithCallId(callId string) *BroadcastSession {
	b.lock.Lock()
	defer b.lock.Unlock()

	session, ok := b.callIds[callId]
	if !ok {
		return nil
	}

	b.RemoveSession(session)
	return session
}

func (b *broadcastManager) Find(sessionId string) *BroadcastSession {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if session, ok := b.sessions[sessionId]; ok {
		return session
	}

	return nil
}

func (b *broadcastManager) AddSession(roomId string, session *BroadcastSession) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	room, ok := b.rooms[roomId]
	if !ok {
		return false
	} else if _, ok := b.sessions[session.Id()]; ok {
		return false
	} else if add := room.Add(session); add {
		b.sessions[session.Id()] = session
		return true
	}

	return false
}

func (b *broadcastManager) AddSessionWithCallId(callId string, session *BroadcastSession) bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.callIds[callId]; !ok {
		b.callIds[callId] = session
	}

	return false
}
