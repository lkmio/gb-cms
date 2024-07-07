package main

import "testing"

func TestName(t *testing.T) {
	for i := 0; i < 10; i++ {
		println(GetLiveSSRC())
		println(GetVodSSRC())
	}
}
