package main

import (
	"fmt"
	"sync"
)

var StreamManager *streamManager

func init() {
	StreamManager = &streamManager{
		streams: make(map[string]*Stream, 64),
		callIds: make(map[string]*Stream, 64),
	}
}

type streamManager struct {
	streams map[string]*Stream
	callIds map[string]*Stream
	lock    sync.RWMutex
}

// Add 添加Stream
// 如果Stream已经存在, 返回oldStream与false
func (s *streamManager) Add(stream *Stream) (*Stream, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	old, ok := s.streams[stream.Id]
	if ok {
		return old, false
	}

	s.streams[stream.Id] = stream
	return nil, true
}

func (s *streamManager) AddWithCallId(stream *Stream) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	id, _ := stream.DialogRequest.CallID()
	if _, ok := s.callIds[id.Value()]; ok {
		return fmt.Errorf("the stream %s has been exist", id.Value())
	}

	s.callIds[id.Value()] = stream
	return nil
}

func (s *streamManager) Find(id string) *Stream {
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

func (s *streamManager) Remove(id string) (*Stream, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	stream, ok := s.streams[id]
	delete(s.streams, id)
	if ok && stream.DialogRequest != nil {
		callID, _ := stream.DialogRequest.CallID()
		delete(s.callIds, callID.Value())
		return stream, nil
	}

	return nil, fmt.Errorf("stream with id %s was not find", id)
}

func (s *streamManager) RemoveWithCallId(id string) (*Stream, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	stream, ok := s.callIds[id]
	if ok {
		delete(s.callIds, id)
		delete(s.streams, stream.Id)
		return stream, nil
	}

	return nil, fmt.Errorf("stream with id %s was not find", id)
}

func (s *streamManager) PopAll() []*Stream {
	s.lock.Lock()
	defer s.lock.Unlock()
	var streams []*Stream

	for _, stream := range s.streams {
		streams = append(streams, stream)
	}

	s.streams = make(map[string]*Stream)
	s.callIds = make(map[string]*Stream)
	return streams
}
