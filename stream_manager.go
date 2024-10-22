package main

import (
	"sync"
)

var StreamManager *streamManager

func init() {
	StreamManager = NewStreamManager()
}

type streamManager struct {
	streams map[StreamID]*Stream
	callIds map[string]*Stream // 本SipUA的CallID与Stream的关系
	lock    sync.RWMutex
}

// Add 添加Stream
// 如果Stream已经存在, 返回oldStream与false
func (s *streamManager) Add(stream *Stream) (*Stream, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	old, ok := s.streams[stream.ID]
	if ok {
		return old, false
	}

	s.streams[stream.ID] = stream
	return nil, true
}

func (s *streamManager) AddWithCallId(id string, stream *Stream) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.callIds[id]; ok {
		return false
	}

	s.callIds[id] = stream
	return true
}

func (s *streamManager) Find(id StreamID) *Stream {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if value, ok := s.streams[id]; ok {
		return value
	}
	return nil
}

func (s *streamManager) FindWithCallId(id string) *Stream {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if value, ok := s.callIds[id]; ok {
		return value
	}
	return nil
}

func (s *streamManager) Remove(id StreamID) *Stream {
	s.lock.Lock()
	defer s.lock.Unlock()

	stream, ok := s.streams[id]
	delete(s.streams, id)
	if ok && stream.DialogRequest != nil {
		callID, _ := stream.DialogRequest.CallID()
		delete(s.callIds, callID.Value())
		return stream
	}

	return nil
}

func (s *streamManager) RemoveWithCallId(id string) *Stream {
	s.lock.Lock()
	defer s.lock.Unlock()

	stream, ok := s.callIds[id]
	if ok {
		delete(s.callIds, id)
		delete(s.streams, stream.ID)
		return stream
	}

	return nil
}

func (s *streamManager) PopAll() []*Stream {
	s.lock.Lock()
	defer s.lock.Unlock()
	var streams []*Stream

	for _, stream := range s.streams {
		streams = append(streams, stream)
	}

	s.streams = make(map[StreamID]*Stream)
	s.callIds = make(map[string]*Stream)
	return streams
}

func NewStreamManager() *streamManager {
	return &streamManager{
		streams: make(map[StreamID]*Stream, 64),
		callIds: make(map[string]*Stream, 64),
	}
}
