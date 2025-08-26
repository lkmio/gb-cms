package stack

import (
	"fmt"
	"gb-cms/common"
	"gb-cms/sdp"
	"strconv"
	"strings"
)

type GBSDP struct {
	SDP                     *sdp.SDP
	SSRC                    string
	Speed                   int
	Media                   *sdp.Media
	MediaType               string
	OfferSetup, AnswerSetup common.SetupType
	StartTime, StopTime     string
	ConnectionAddr          string
	IsTcpTransport          bool
}

func ParseGBSDP(body string) (*GBSDP, error) {
	offer, err := sdp.Parse(body)
	if err != nil {
		return nil, err
	}

	gbSdp := &GBSDP{SDP: offer}
	// 解析设置下载速度
	var setup string
	for _, attr := range offer.Attrs {
		if "downloadspeed" == attr[0] {
			speed, err := strconv.Atoi(attr[1])
			if err != nil {
				return nil, err
			}
			gbSdp.Speed = speed
		} else if "setup" == attr[0] {
			setup = attr[1]
		}
	}

	// 解析ssrc
	for _, attr := range offer.Other {
		if "y" == attr[0] {
			gbSdp.SSRC = attr[1]
		}
	}

	if offer.Video != nil {
		gbSdp.Media = offer.Video
		gbSdp.MediaType = "video"
	} else if offer.Audio != nil {
		gbSdp.Media = offer.Audio
		gbSdp.MediaType = "audio"
	}

	tcp := strings.HasPrefix(gbSdp.Media.Proto, "TCP")
	if "passive" == setup && tcp {
		gbSdp.OfferSetup = common.SetupTypePassive
		gbSdp.AnswerSetup = common.SetupTypeActive
	} else if "active" == setup && tcp {
		gbSdp.OfferSetup = common.SetupTypeActive
		gbSdp.AnswerSetup = common.SetupTypePassive
	}

	time := strings.Split(gbSdp.SDP.Time, " ")
	if len(time) < 2 {
		return nil, fmt.Errorf("sdp的时间范围格式错误 time: %s sdp: %s", gbSdp.SDP.Time, body)
	}

	gbSdp.StartTime = time[0]
	gbSdp.StopTime = time[1]
	gbSdp.IsTcpTransport = tcp
	gbSdp.ConnectionAddr = fmt.Sprintf("%s:%d", gbSdp.SDP.Addr, gbSdp.Media.Port)
	return gbSdp, nil
}
