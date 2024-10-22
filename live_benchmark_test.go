package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func request(url string, body []byte) (*http.Response, error) {
	client := &http.Client{}
	request, err := http.NewRequest("post", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}

func queryAllDevices() []Device {
	response, err := request("http://localhost:9000/api/v1/device/list", nil)
	if err != nil {
		panic(err)
	}

	all, err := io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	v := struct {
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		Data []Device `json:"data,omitempty"`
	}{}

	err = json.Unmarshal(all, &v)
	if err != nil {
		panic(err)
	}

	return v.Data
}

func startLive(deviceId, channelId, setup string) (bool, string) {
	params := map[string]string{
		"device_id":  deviceId,
		"channel_id": channelId,
		"setup":      setup,
	}

	requestBody, err := json.Marshal(params)
	if err != nil {
		panic(err)
	}

	response, err := request("http://localhost:9000/api/v1/live/start", requestBody)
	if err != nil {
		panic(err)
	}

	if response.StatusCode != 200 {
		return false, ""
	}

	all, err := io.ReadAll(response.Body)
	if len(all) == 0 {
		return true, ""
	}

	v := struct {
		Code int               `json:"code"`
		Msg  string            `json:"msg"`
		Data map[string]string `json:"data,omitempty"`
	}{}

	err = json.Unmarshal(all, &v)
	if err != nil {
		panic(err)
	}

	return true, v.Data["stream_id"]
}

func startLiveAll(setup string) {
	devices := queryAllDevices()
	if len(devices) == 0 {
		return
	}

	max := 50
	for _, device := range devices {
		for _, channel := range device.Channels {
			go startLive(device.ID, channel.DeviceID, setup)
			max--
			if max < 1 {
				return
			}
		}
	}
}

func TestLiveAll(t *testing.T) {
	index := 0

	for {
		index++
		var setup string

		if index%1 == 0 {
			setup = "udp"
		} else if index%2 == 0 {
			setup = "passive"
		} else if index%3 == 0 {
			setup = "active"
		} else if index%4 == 0 {
			//关闭所有流，再请求
		}

		go startLiveAll(setup)

		time.Sleep(60 * time.Second)
	}
}
