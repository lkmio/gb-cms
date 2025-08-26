package common

import (
	"github.com/lkmio/avformat/utils"
	"strings"
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
	if InviteTypePlayback == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".playback" + "." + startTime + "." + endTime)
	} else if InviteTypeDownload == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".download" + "." + startTime + "." + endTime)
	} else if InviteTypeBroadcast == inviteType {
		return StreamID(strings.Join(streamId, "/") + ".broadcast")
	}

	return StreamID(strings.Join(streamId, "/"))
}
