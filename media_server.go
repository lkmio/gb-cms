package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func Send(path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/%s", Config.MediaServer, path)

	marshal, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	request, err := http.NewRequest("post", url, bytes.NewBuffer(marshal))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}

func CreateGBSource(id, setup string, ssrc uint32) (string, uint16, error) {
	v := &struct {
		Source string `json:"source"`
		Setup  string `json:"setup"`
		SSRC   uint32 `json:"ssrc"`
	}{
		Source: id,
		Setup:  setup,
		SSRC:   ssrc,
	}

	response, err := Send("api/v1/gb28181/source/create", v)
	if err != nil {
		return "", 0, err
	}

	connectInfo := &struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			IP   string `json:"ip"`
			Port uint16 `json:"port,omitempty"`
		}
	}{}

	err = DecodeJSONBody(response.Body, connectInfo)
	if err != nil {
		return "", 0, err
	}

	return connectInfo.Data.IP, connectInfo.Data.Port, nil
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

func CloseGBSource(id string) error {
	v := &struct {
		Source string `json:"source"`
	}{
		Source: id,
	}

	_, err := Send("api/v1/gb28181/source/close", v)
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

	r := struct {
		ID   string `json:"id"` //sink id
		IP   string `json:"ip"`
		Port uint16 `json:"port"`
	}{}

	if err = DecodeJSONBody(response.Body, &r); err != nil {
		return "", 0, "", err
	}

	return r.IP, r.Port, r.ID, nil
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
