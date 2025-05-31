package hook

import (
	"bytes"
	"encoding/json"
	"net/http"
)

const (
	EventTypeDeviceOnline = iota + 1
	EventTypeDeviceOffline
	EventTypeDevicePosition
	EventTypeDeviceOnInvite
)

var (
	EventUrls = make(map[int]string)
)

func RegisterEventUrl(event int, url string) {
	EventUrls[event] = url
}

func PostEvent(url string, body []byte) (*http.Response, error) {
	client := &http.Client{
		//Timeout: time.Duration(AppConfig.Hooks.Timeout),
	}

	request, err := http.NewRequest("post", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	return client.Do(request)
}

func PostOnInviteEvent(simNumber, channelNumber string) (*http.Response, error) {
	params := map[string]string{
		"sim_number":     simNumber,
		"channel_number": channelNumber,
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return PostEvent(EventUrls[EventTypeDeviceOnInvite], body)
}
