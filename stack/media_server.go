package stack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gb-cms/common"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	TransStreamRtmp       = iota + 1
	TransStreamFlv        = 2
	TransStreamRtsp       = 3
	TransStreamHls        = 4
	TransStreamRtc        = 5
	TransStreamGBCascaded = 6 // 国标级联转发
	TransStreamGBTalk     = 7 // 国标广播/对讲转发
	TransStreamGBGateway  = 8 // 国标网关
)

const (
	SourceTypeRtmp = iota + 1
	SourceType28181
	SourceType1078
	SourceTypeGBTalk
)

type SourceDetails struct {
	ID        string    `json:"id"`
	Protocol  string    `json:"protocol"`   // 推流协议
	Time      time.Time `json:"time"`       // 推流时间
	SinkCount int       `json:"sink_count"` // 播放端计数
	Bitrate   string    `json:"bitrate"`    // 码率统计
	Tracks    []string  `json:"tracks"`     // 每路流编码器ID
	Urls      []string  `json:"urls"`       // 拉流地址
}

type SinkDetails struct {
	ID       string    `json:"id"`
	Protocol string    `json:"protocol"` // 拉流协议
	Time     time.Time `json:"time"`     // 拉流时间
	Bitrate  string    `json:"bitrate"`  // 码率统计
	Tracks   []string  `json:"tracks"`   // 每路流编码器ID
}

type SDP struct {
	SessionName string `json:"session_name,omitempty"` // play/download/playback/talk/broadcast
	Addr        string `json:"addr,omitempty"`         // 连接地址
	SSRC        string `json:"ssrc,omitempty"`
	Setup       string `json:"setup,omitempty"`     // active/passive
	Transport   string `json:"transport,omitempty"` // tcp/udp
	Speed       int    `json:"speed"`
	StartTime   int    `json:"start_time,omitempty"`
	EndTime     int    `json:"end_time,omitempty"`
	FileSize    int    `json:"file_size,omitempty"`
}

type SourceSDP struct {
	Source string `json:"source"` // GetSourceID
	SDP
}

type GBOffer struct {
	SourceSDP
	AnswerSetup         string `json:"answer_setup,omitempty"` // 希望应答的连接方式
	TransStreamProtocol int    `json:"trans_stream_protocol,omitempty"`
}

func Send(path string, body interface{}) (*http.Response, error) {
	return SendWithUrlParams(path, body, nil)
}

func SendWithUrlParams(path string, body interface{}, values url.Values) (*http.Response, error) {
	if values != nil {
		params := values.Encode()
		if len(params) > 0 {
			path = fmt.Sprintf("%s?%s", path, params)
		}
	}

	url := fmt.Sprintf("%s/%s", common.Config.MediaServer, path)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	request, err := http.NewRequest("post", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}

func MSCreateGBSource(id, setup string, ssrc string, sessionName string, speed int) (string, uint16, []string, string, error) {
	v := &SourceSDP{
		Source: id,
		SDP: SDP{
			Setup:       setup,
			SSRC:        ssrc,
			SessionName: sessionName,
			Speed:       speed,
		},
	}

	response, err := Send("api/v1/gb28181/source/create", v)
	if err != nil {
		return "", 0, nil, "", err
	}

	data := &common.Response[struct {
		SDP
		Urls []string `json:"urls"`
	}]{}

	if err = common.DecodeJSONBody(response.Body, data); err != nil {
		return "", 0, nil, "", err
	} else if http.StatusOK != data.Code {
		return "", 0, nil, "", fmt.Errorf(data.Msg)
	}

	host, p, err := net.SplitHostPort(data.Data.Addr)
	if err != nil {
		return "", 0, nil, "", err
	}

	port, err := strconv.Atoi(p)
	return host, uint16(port), data.Data.Urls, data.Data.SSRC, err
}

func MSConnectGBSource(id, addr string, fileSize int) error {
	v := &SourceSDP{
		Source: id,
		SDP: SDP{
			Addr:     addr,
			FileSize: fileSize,
		},
	}

	_, err := Send("api/v1/gb28181/answer/set", v)
	return err
}

func MSCloseSource(id string) error {
	v := &struct {
		Source string `json:"source"`
	}{
		Source: id,
	}

	_, err := Send("api/v1/source/close", v)
	return err
}

func MSCloseSink(sourceId string, sinkId string) {
	v := struct {
		SourceID string `json:"source"`
		SinkID   string `json:"sink"` // sink id
	}{
		sourceId, sinkId,
	}

	_, _ = Send("api/v1/sink/close", v)
}

func MSQuerySourceList() ([]*SourceDetails, error) {
	response, err := Send("api/v1/source/list", nil)
	if err != nil {
		return nil, err
	}

	data := &common.Response[[]*SourceDetails]{}
	if err = common.DecodeJSONBody(response.Body, data); err != nil {
		return nil, err
	}

	return data.Data, err
}

func MSQuerySinkList(source string) ([]*SinkDetails, error) {
	id := struct {
		Source string `json:"source"`
	}{source}

	response, err := Send("api/v1/sink/list", id)
	if err != nil {
		return nil, err
	}

	data := &common.Response[[]*SinkDetails]{}
	if err = common.DecodeJSONBody(response.Body, data); err != nil {
		return nil, err
	}

	return data.Data, err
}

func MSAddForwardSink(protocol int, source, addr, offerSetup, answerSetup, ssrc, sessionName string, values url.Values) (string, uint16, string, string, error) {
	offer := &GBOffer{
		SourceSDP: SourceSDP{
			Source: source,
			SDP: SDP{
				Addr:        addr,
				Setup:       offerSetup,
				SSRC:        ssrc,
				SessionName: sessionName,
			},
		},
		AnswerSetup:         answerSetup,
		TransStreamProtocol: protocol,
	}

	var err error
	response, err := SendWithUrlParams("api/v1/sink/add", offer, values)
	if err != nil {
		return "", 0, "", "", err
	}

	data := &common.Response[struct {
		Sink string `json:"sink"`
		SDP
	}]{}

	if err = common.DecodeJSONBody(response.Body, data); err != nil {
		return "", 0, "", "", err
	} else if http.StatusOK != data.Code {
		return "", 0, "", "", fmt.Errorf(data.Msg)
	}

	host, p, err := net.SplitHostPort(data.Data.Addr)
	if err != nil {
		return "", 0, "", "", err
	}

	port, _ := strconv.Atoi(p)
	return host, uint16(port), data.Data.Sink, data.Data.SSRC, nil
}

func MSQueryStreamInfo(header http.Header, queryParams string) (*http.Response, error) {
	// 构建目标URL
	targetURL := common.Config.MediaServer + "/api/v1/stream/info"
	if queryParams != "" {
		targetURL += "?" + queryParams
	}

	// 创建转发请求
	proxyReq, err := http.NewRequest("POST", targetURL, nil)
	if err != nil {
		return nil, err
	}

	// 复制请求头
	for name, values := range header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return client.Do(proxyReq)
}
