package main

import (
	"context"
	"github.com/ghettovoice/gosip/sip"
	"time"
)

type Stream struct {
	Id            string //推流ID
	Protocol      string //推流协议
	DialogRequest sip.Request

	publishEvent chan byte
	cancelFunc   func()
}

func (s *Stream) waitPublishStream() bool {
	s.publishEvent = make(chan byte, 0)
	timeout, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	s.cancelFunc = cancelFunc

	select {
	case <-s.publishEvent:
		return true
	case <-timeout.Done():
		s.cancelFunc = nil
		return false
	}
}

func (s *Stream) Close(sendBye bool) {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	if sendBye && s.DialogRequest != nil {
		SipUA.SendRequest(s.CreateRequestFromDialog(sip.BYE))
		s.DialogRequest = nil
	}
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
