package main

import (
	"context"
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"sync"
	"time"
)

var (
	Dialogs = NewDialogManager[*StreamWaiting]()
)

type StreamWaiting struct {
	onPublishCb chan int // 等待推流hook的管道
	cancelFunc  func()   // 取消等待推流hook的ctx
	data        interface{}
}

func (s *StreamWaiting) Receive(seconds int) int {
	s.onPublishCb = make(chan int, 0)
	timeout, cancelFunc := context.WithTimeout(context.Background(), time.Duration(seconds)*time.Second)
	s.cancelFunc = cancelFunc
	select {
	case code := <-s.onPublishCb:
		return code
	case <-timeout.Done():
		s.cancelFunc = nil
		return -1
	}
}
func (s *StreamWaiting) Put(code int) {
	s.onPublishCb <- code
}

type DialogManager[T any] struct {
	lock    sync.RWMutex
	dialogs map[string]T
}

func (d *DialogManager[T]) Add(id string, dialog T) (T, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()

	var old T
	var ok bool
	if old, ok = d.dialogs[id]; ok {
		return old, false
	}

	d.dialogs[id] = dialog
	return old, true
}

func (d *DialogManager[T]) Find(id string) T {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.dialogs[id]
}

func (d *DialogManager[T]) Remove(id string) T {
	d.lock.Lock()
	defer d.lock.Unlock()
	dialog := d.dialogs[id]
	delete(d.dialogs, id)
	return dialog
}

func UnmarshalDialog(dialog string) (sip.Request, error) {
	packetParser := parser.NewPacketParser(logger)
	message, err := packetParser.ParseMessage([]byte(dialog))
	if err != nil {
		return nil, err
	} else if request := message.(sip.Request); request == nil {
		return nil, fmt.Errorf("dialog message is not sip request")
	} else {
		return request, nil
	}
}

func NewDialogManager[T any]() *DialogManager[T] {
	return &DialogManager[T]{
		dialogs: make(map[string]T),
	}
}
