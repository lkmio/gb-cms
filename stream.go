package main

import (
	"context"
	"github.com/ghettovoice/gosip/sip"
	"sync/atomic"
	"time"
)

type Stream struct {
	Id            string //推流ID
	Protocol      string //推流协议
	DialogRequest sip.Request
	StreamType    InviteType

	sinkCount    int32
	publishEvent chan byte
	cancelFunc   func()
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

	if sendBye && s.DialogRequest != nil {
		SipUA.SendRequest(s.CreateRequestFromDialog(sip.BYE))
		s.DialogRequest = nil
	}

	go CloseGBSource(s.Id)
}

func (s *Stream) CreateRequestFromDialog(method sip.RequestMethod) sip.Request {
	{
		seq, _ := s.DialogRequest.CSeq()
		seq.SeqNo++
		seq.MethodName = method
	}

	request := s.DialogRequest.Clone().(sip.Request)
	request.SetMethod(method)

	return request
}
