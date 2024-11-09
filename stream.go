package main

import (
	"context"
	"github.com/ghettovoice/gosip/sip"
	"sync"
	"sync/atomic"
	"time"
)

// Sink 级联转发
type Sink struct {
	id       string
	deviceID string
	dialog   sip.Request

	platformID string // 级联上级ID
}

// Stream 国标推流源
type Stream struct {
	ID            StreamID // 推流ID
	DialogRequest sip.Request

	sinkCount    int32 // 拉流数量+级联转发数量
	publishEvent chan byte
	cancelFunc   func()

	forwardSinks map[string]*Sink // 级联转发Sink, Key为与上级的CallID
	lock         sync.RWMutex
	urls         []string // 拉流地址
}

func (s *Stream) AddForwardSink(id string, sink *Sink) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.forwardSinks[id] = sink
}

func (s *Stream) RemoveForwardSink(id string) *Sink {
	s.lock.Lock()
	defer s.lock.Unlock()

	sink, ok := s.forwardSinks[id]
	if ok {
		delete(s.forwardSinks, id)
	}

	return sink
}

func (s *Stream) ForwardSinks() []*Sink {
	s.lock.Lock()
	defer s.lock.Unlock()

	var sinks []*Sink
	for _, sink := range s.forwardSinks {
		sinks = append(sinks, sink)
	}

	return sinks
}

func (s *Stream) WaitForPublishEvent(seconds int) bool {
	s.publishEvent = make(chan byte, 0)
	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	s.cancelFunc = cancelFunc

	select {
	case <-s.publishEvent:
		return true
	case <-timeout.Done():
		s.cancelFunc = nil
		return false
	}
}

func (s *Stream) SinkCount() int32 {
	return atomic.LoadInt32(&s.sinkCount)
}

func (s *Stream) IncreaseSinkCount() int32 {
	return atomic.AddInt32(&s.sinkCount, 1)
}

func (s *Stream) DecreaseSinkCount() int32 {
	return atomic.AddInt32(&s.sinkCount, -1)
}

func (s *Stream) Close(sendBye bool) {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	// 断开与下级的会话
	if sendBye && s.DialogRequest != nil {
		SipUA.SendRequest(s.CreateRequestFromDialog(sip.BYE))
		s.DialogRequest = nil
	}

	go CloseGBSource(string(s.ID))

	// 关闭所有级联会话
	sinks := s.ForwardSinks()
	for _, sink := range sinks {
		platform := PlatformManager.FindPlatform(sink.deviceID)
		id, _ := sink.dialog.CallID()
		platform.CloseStream(id.Value(), true, true)
	}
}

func CreateRequestFromDialog(dialog sip.Request, method sip.RequestMethod) sip.Request {
	{
		seq, _ := dialog.CSeq()
		seq.SeqNo++
		seq.MethodName = method
	}

	request := dialog.Clone().(sip.Request)
	request.SetMethod(method)
	request.SetSource("")
	return request
}

func (s *Stream) CreateRequestFromDialog(method sip.RequestMethod) sip.Request {
	return CreateRequestFromDialog(s.DialogRequest, method)
}
