package stack

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"net"
	"strconv"
	"strings"
)

const (
	XmlHeaderGBK = `<?xml version="1.0" encoding="GB2312"?>` + "\r\n"
)

func BuildSDP(media, userName, sessionName, ip string, port uint16, startTime, stopTime, setup string, speed int, ssrc string, attrs ...string) string {
	format := "v=0\r\n" +
		"o=%s 0 0 IN IP4 %s\r\n" +
		"s=%s\r\n" +
		"c=IN IP4 %s\r\n" +
		"t=%s %s\r\n" +
		"m=%s %d %s %s\r\n" +
		"a=%s\r\n"

	tcpFormat := "a=setup:%s\r\n" +
		"a=connection:new\r\n"

	var tcp bool
	var mediaProtocol string
	if "active" == setup || "passive" == setup {
		mediaProtocol = "TCP/RTP/AVP"
		tcp = true
	} else {
		mediaProtocol = "RTP/AVP"
	}

	var mediaFormats []string
	for _, attr := range attrs {
		mediaFormats = append(mediaFormats, strings.Split(attr, " ")[0])
	}

	sdp := fmt.Sprintf(format, userName, ip, sessionName, ip, startTime, stopTime, media, port, mediaProtocol, strings.Join(mediaFormats, " "), "recvonly")
	for _, attr := range attrs {
		sdp += fmt.Sprintf("a=rtpmap:%s\r\n", attr)
	}

	if tcp {
		sdp += fmt.Sprintf(tcpFormat, setup)
	}

	if speed > 0 {
		sdp += fmt.Sprintf("a=downloadspeed:%d\r\n", speed)
	}

	sdp += fmt.Sprintf("y=%s\r\n", ssrc)
	return sdp
}

func NewSIPRequestBuilderWithTransport(transport string) *sip.RequestBuilder {
	builder := sip.NewRequestBuilder()
	hop := sip.ViaHop{
		Transport: transport,
	}

	builder.AddVia(&hop)
	builder.SetUserAgent(nil)
	return builder
}

func BuildMessageRequest(from, fromRealm, to, toAddr, transport, body string) (sip.Request, error) {
	if !strings.HasPrefix(body, "<?xml") {
		body = XmlHeaderGBK + body
	}

	//gbkBody, _, err := transform.String(simplifiedchinese.GBK.NewEncoder(), body)
	//if err != nil {
	//	panic(err)
	//}
	gbkBody := body

	builder := NewRequestBuilder(sip.MESSAGE, from, fromRealm, to, toAddr, transport)
	builder.SetContentType(&XmlMessageType)
	builder.SetBody(gbkBody)
	return builder.Build()
}

func NewRequestBuilder(method sip.RequestMethod, fromUser, realm, toUser, toAddr, transport string) *sip.RequestBuilder {
	builder := NewSIPRequestBuilderWithTransport(transport)
	builder.SetMethod(method)

	host, p, _ := net.SplitHostPort(toAddr)
	port, _ := strconv.Atoi(p)
	sipPort := sip.Port(port)

	requestUri := &sip.SipUri{
		FUser: sip.String{Str: toUser},
		FHost: host,
		FPort: &sipPort,
	}

	builder.SetRecipient(requestUri)

	fromAddress := &sip.Address{
		Uri: &sip.SipUri{
			FUser: sip.String{Str: fromUser},
			FHost: realm,
		},
	}

	fromAddress.Params = sip.NewParams().Add("tag", sip.String{Str: GenerateTag()})
	builder.SetFrom(fromAddress)
	builder.SetTo(&sip.Address{
		Uri: requestUri,
	})

	return builder
}
