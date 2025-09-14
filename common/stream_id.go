package common

import (
	"github.com/lkmio/avformat/utils"
	"strconv"
	"strings"
	"time"
)

type StreamID string // 目前目涉及转码，多路流, 与SourceID相同

func (s StreamID) DeviceID() string {
	return strings.Split(string(s), "/")[0]
}

func (s StreamID) ChannelID() string {
	return strings.Split(strings.Split(string(s), "/")[1], ".")[0]
}

func GenerateStreamID(inviteType InviteType, deviceId, channelId string, startTime, endTime string) StreamID {
	utils.Assert(channelId != "")

	var streamId []string
	if deviceId != "" {
		streamId = append(streamId, deviceId)
	}

	streamId = append(streamId, channelId)

	// 转换时间戳
	if startTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", startTime); err == nil {
			startTime = strconv.FormatInt(t.Unix(), 10)
		}
	}
	if endTime != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", endTime); err == nil {
			endTime = strconv.FormatInt(t.Unix(), 10)
		}
	}

	if InviteTypePlayback == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".playback" + "." + startTime + "." + endTime)
	} else if InviteTypeDownload == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".download" + "." + startTime + "." + endTime)
	} else if InviteTypeBroadcast == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".broadcast")
	}

	return StreamID(strings.Join(streamId, "/"))
}
