package main

import (
	"fmt"
	"sync"
)

const (
	SsrcMaxValue = 999999999
)

var (
	ssrcCount uint32
	lock      sync.Mutex
)

func NextSSRC() uint32 {
	lock.Lock()
	defer lock.Unlock()
	ssrcCount = (ssrcCount + 1) % SsrcMaxValue
	return ssrcCount
}

func GetLiveSSRC() string {
	return fmt.Sprintf("0%09d", NextSSRC())
}

func GetVodSSRC() string {
	return fmt.Sprintf("%d", 1000000000+NextSSRC())
}
