package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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

func Send(path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/%s", Config.MediaServer, path)

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

func CreateGBSource(id, setup string, ssrc uint32, inviteType int) (string, uint16, []string, string, error) {
	v := &struct {
		Source string `json:"source"`
		Setup  string `json:"setup"`
		SSRC   uint32 `json:"ssrc"`
		Type   int    `json:"type"`
	}{
		Source: id,
		Setup:  setup,
		SSRC:   ssrc,
		Type:   inviteType,
	}

	response, err := Send("api/v1/gb28181/source/create", v)
	if err != nil {
		return "", 0, nil, "", err
	}

	data := &Response[struct {
		IP   string   `json:"ip"`
		Port uint16   `json:"port,omitempty"`
		Urls []string `json:"urls"`
		SSRC string   `json:"ssrc,omitempty"`
	}]{}

	if err = DecodeJSONBody(response.Body, data); err != nil {
		return "", 0, nil, "", err
	} else if http.StatusOK != data.Code {
		return "", 0, nil, "", fmt.Errorf(data.Msg)
	}

	return data.Data.IP, data.Data.Port, data.Data.Urls, data.Data.SSRC, nil
}

func ConnectGBSource(id, addr string) error {
	v := &struct {
		Source     string `json:"source"` //SourceID
		RemoteAddr string `json:"remote_addr"`
	}{
		Source:     id,
		RemoteAddr: addr,
	}

	_, err := Send("api/v1/gb28181/source/connect", v)
	return err
}

func CloseSource(id string) error {
	v := &struct {
		Source string `json:"source"`
	}{
		Source: id,
	}

	_, err := Send("api/v1/source/close", v)
	return err
}

func AddForwardStreamSink(id, serverAddr, setup string, ssrc uint32) (ip string, port uint16, sinkId string, err error) {
	v := struct {
		Source string `json:"source"`
		Addr   string `json:"addr"`
		Setup  string `json:"setup"`
		SSRC   uint32 `json:"ssrc"`
	}{
		Source: id,
		Addr:   serverAddr,
		Setup:  setup,
		SSRC:   ssrc,
	}

	response, err := Send("api/v1/gb28181/forward", v)
	if err != nil {
		return "", 0, "", err
	}

	data := &Response[struct {
		Sink string `json:"sink"`
		IP   string `json:"ip"`
		Port uint16 `json:"port"`
	}]{}

	if err = DecodeJSONBody(response.Body, data); err != nil {
		return "", 0, "", err
	} else if http.StatusOK != data.Code {
		return "", 0, "", fmt.Errorf(data.Msg)
	}

	return data.Data.IP, data.Data.Port, data.Data.Sink, nil
}

func CloseSink(sourceId string, sinkId string) {
	v := struct {
		SourceID string `json:"source"`
		SinkID   string `json:"sink"` // sink id
	}{
		sourceId, sinkId,
	}

	_, _ = Send("api/v1/sink/close", v)
}

func QuerySourceList() ([]*SourceDetails, error) {
	response, err := Send("api/v1/source/list", nil)
	if err != nil {
		return nil, err
	}

	data := &Response[[]*SourceDetails]{}
	if err = DecodeJSONBody(response.Body, data); err != nil {
		return nil, err
	}

	return data.Data, err
}

func QuerySinkList(source string) ([]*SinkDetails, error) {
	id := struct {
		Source string `json:"source"`
	}{source}

	response, err := Send("api/v1/sink/list", id)
	if err != nil {
		return nil, err
	}

	data := &Response[[]*SinkDetails]{}
	if err = DecodeJSONBody(response.Body, data); err != nil {
		return nil, err
	}

	return data.Data, err
}
