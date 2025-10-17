package stack

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
)

type Event string

func (ev *Event) String() string { return fmt.Sprintf("%s: %s", ev.Name(), ev.Value()) }

func (ev *Event) Name() string { return "Event" }

func (ev Event) Value() string { return string(ev) }

func (ev *Event) Clone() sip.Header { return ev }

func (ev *Event) Equals(other interface{}) bool {
	if h, ok := other.(Event); ok {
		if ev == nil {
			return false
		}

		return *ev == h
	}
	if h, ok := other.(*Event); ok {
		if ev == h {
			return true
		}
		if ev == nil && h != nil || ev != nil && h == nil {
			return false
		}

		return *ev == *h
	}

	return false
}

type SubscriptionState string

func (ev *SubscriptionState) String() string { return fmt.Sprintf("%s: %s", ev.Name(), ev.Value()) }

func (ev *SubscriptionState) Name() string { return "Subscription-State" }

func (ev SubscriptionState) Value() string { return string(ev) }

func (ev *SubscriptionState) Clone() sip.Header { return ev }

func (ev *SubscriptionState) Equals(other interface{}) bool {
	if h, ok := other.(SubscriptionState); ok {
		if ev == nil {
			return false
		}

		return *ev == h
	}
	if h, ok := other.(*SubscriptionState); ok {
		if ev == h {
			return true
		}
		if ev == nil && h != nil || ev != nil && h == nil {
			return false
		}

		return *ev == *h
	}

	return false
}

//const (
//	SubscriptionStateActive  SubscriptionState = "active"
//	SubscriptionStatePending SubscriptionState = "pending"
//	SubscriptionStateTerminated SubscriptionState = "terminated"
//)
