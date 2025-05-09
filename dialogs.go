package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/ghettovoice/gosip/sip/parser"
	"sync"
)

type DialogManager[T any] struct {
	lock    sync.RWMutex
	dialogs map[string]T
	callIds map[string]T
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

func (d *DialogManager[T]) AddWithCallId(id string, dialog T) bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	if _, ok := d.callIds[id]; ok {
		return false
	}

	d.callIds[id] = dialog
	return true
}

func (d *DialogManager[T]) Find(id string) T {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.dialogs[id]
}

func (d *DialogManager[T]) FindWithCallId(id string) T {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.callIds[id]
}

func (d *DialogManager[T]) Remove(id string) T {
	d.lock.Lock()
	defer d.lock.Unlock()
	dialog := d.dialogs[id]
	delete(d.dialogs, id)
	return dialog
}

func (d *DialogManager[T]) RemoveWithCallId(id string) T {
	d.lock.Lock()
	defer d.lock.Unlock()
	dialog := d.callIds[id]
	delete(d.callIds, id)
	return dialog
}

func (d *DialogManager[T]) All() []T {
	d.lock.RLock()
	defer d.lock.RUnlock()
	var result []T
	for _, v := range d.dialogs {
		result = append(result, v)
	}
	return result
}

func (d *DialogManager[T]) PopAll() []T {
	d.lock.Lock()
	defer d.lock.Unlock()
	var result []T
	for _, v := range d.dialogs {
		result = append(result, v)
	}

	d.dialogs = make(map[string]T)
	return result
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
		callIds: make(map[string]T),
	}
}
