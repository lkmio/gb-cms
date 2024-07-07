package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
)

type Subject string

func (subject Subject) String() string {
	return fmt.Sprintf("%s: %s", subject.Name(), subject.Value())
}

func (subject Subject) Name() string { return "Subject" }

func (subject Subject) Value() string { return string(subject) }

func (subject Subject) Clone() sip.Header { return subject }

func (subject Subject) Equals(other interface{}) bool {
	//if h, ok := other.(Subject); ok {
	//	return subject == h
	//}
	//if h, ok := other.(*Subject); ok {
	//	if subject == h {
	//		return true
	//	}
	//	if subject == nil && h != nil || subject != nil && h == nil {
	//		return false
	//	}
	//
	//	return *subject == *h
	//}

	return false
}
