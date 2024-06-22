package main

import (
	"sync"
)

const (
	SsrcMaxValue = 999999999
)

var (
	ssrc uint32
	lock sync.Mutex
)

func GetLiveSSRC() uint32 {
	lock.Lock()
	defer lock.Unlock()
	ssrc = (ssrc + 1) % SsrcMaxValue
	return ssrc
}

func GetVodSSRC() uint32 {
	return 1000000000 + GetLiveSSRC()
}
