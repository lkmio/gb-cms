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
	streams sync.Map
	callIds sync.Map
}

func (s *streamManager) Add(device *Stream) error {
	if _, ok := s.streams.LoadOrStore(device.Id, device); ok {
		return fmt.Errorf("the stream %s has been exist", device.Id)
	}

	return nil
}

func (s *streamManager) AddWithCallId(device *Stream) error {
	id, _ := device.ByeRequest.CallID()
	if _, ok := s.streams.LoadOrStore(id.Value(), device); ok {
		return fmt.Errorf("the stream %s has been exist", id.Value())
	}

	return nil
}

func (s *streamManager) Find(id string) *Stream {
	if value, ok := s.streams.Load(id); ok {
		return value.(*Stream)
	}

	return nil
}

func (s *streamManager) FindWithCallId(id string) *Stream {
	if value, ok := s.callIds.Load(id); ok {
		return value.(*Stream)
	}

	return nil
}

func (s *streamManager) Remove(id string) (*Stream, error) {
	value, loaded := s.streams.LoadAndDelete(id)
	if loaded && value.(*Stream).ByeRequest != nil {
		id, _ := value.(*Stream).ByeRequest.CallID()
		s.callIds.Delete(id.Value())
		return value.(*Stream), nil
	}

	return nil, fmt.Errorf("stream with id %s was not find", id)
}

func (s *streamManager) RemoveWithCallId(id string) (*Stream, error) {
	value, loaded := s.callIds.LoadAndDelete(id)
	if loaded {
		s.streams.Delete(value.(*Stream).Id)
		return value.(*Stream), nil
	}

	return nil, fmt.Errorf("stream with id %s was not find", id)
}
