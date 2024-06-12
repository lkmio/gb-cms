package main

import (
	"fmt"
	"sync"
)

var StreamManager *streamManager

func init() {
	StreamManager = &streamManager{}
}

type streamManager struct {
	m sync.Map
}

func (s *streamManager) Add(device *Stream) error {
	_, ok := s.m.LoadOrStore(device.Id, device)
	if ok {
		return fmt.Errorf("the stream %s has been exist", device.Id)
	}

	return nil
}

func (s *streamManager) Find(id string) *Stream {
	value, ok := s.m.Load(id)
	if ok {
		return value.(*Stream)
	}

	return nil
}

func (s *streamManager) Remove(id string) (*Stream, error) {
	value, loaded := s.m.LoadAndDelete(id)
	if loaded {
		return value.(*Stream), nil
	}

	return nil, fmt.Errorf("stream with id %s was not find", id)
}
