package common

type OnlineStatus string

const (
	ON  = OnlineStatus("ON")
	OFF = OnlineStatus("OFF")
)

func (s OnlineStatus) String() string {
	return string(s)
}

type SetupType int

const (
	SetupTypeUDP SetupType = iota + 1
	SetupTypePassive
	SetupTypeActive
)

var (
	DefaultSetupType = SetupTypePassive
)

func (s SetupType) String() string {
	switch s {
	case SetupTypePassive:
		return "passive"
	case SetupTypeActive:
		return "active"
	default:
		return "udp"
	}
}

func (s SetupType) MediaProtocol() string {
	switch s {
	case SetupTypePassive, SetupTypeActive:
		return "TCP/RTP/AVP"
	default:
		return "RTP/AVP"
	}
}

func (s SetupType) Transport() string {
	switch s {
	case SetupTypePassive, SetupTypeActive:
		return "TCP"
	default:
		return "UDP"
	}
}

type InviteType string

const (
	InviteTypePlay      = InviteType("play")
	InviteTypePlayback  = InviteType("playback")
	InviteTypeDownload  = InviteType("download")
	InviteTypeBroadcast = InviteType("broadcast")
	InviteTypeTalk      = InviteType("talk")
)

func (i *InviteType) SessionName2Type(name string) {
	switch name {
	case "download":
		*i = InviteTypeDownload
		break
	case "playback":
		*i = InviteTypePlayback
		break
	//case "play":
	default:
		*i = InviteTypePlay
		break
	}
}
