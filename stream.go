package main

import (
	"context"
	"github.com/ghettovoice/gosip/sip"
	"time"
)

type Stream struct {
	Id         string //推流ID
	Protocol   string //推流协议
	ByeRequest sip.Request

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

func (s *Stream) Close() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	if s.ByeRequest != nil {
		SipUA.SendRequest(s.ByeRequest)
		s.ByeRequest = nil
	}
}
