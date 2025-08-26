package common

import (
	"encoding/json"
	"net/http"
)

type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func HttpResponse(w http.ResponseWriter, code int, msg string) error {
	return HttpResponseJson(w, MalformedRequest{
		Code: code,
		Msg:  msg,
	})
}

func HttpResponseJson(w http.ResponseWriter, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return HttpResponseJsonStr(w, string(body))
}

func HttpResponseJsonStr(w http.ResponseWriter, payload string) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT")

	_, err := w.Write([]byte(payload))
	return err
}

func HttpResponseOK(w http.ResponseWriter, data interface{}) error {
	return HttpResponseJson(w, MalformedRequest{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: data,
	})
}

func HttpResponseSuccess(w http.ResponseWriter, data interface{}) error {
	return HttpResponseJson(w, MalformedRequest{
		Code: http.StatusOK,
		Msg:  "Success",
		Data: data,
	})
}

func HttpResponseError(w http.ResponseWriter, msg string) error {
	return HttpResponseJson(w, MalformedRequest{
		Code: -1,
		Msg:  msg,
		Data: nil,
	})
}
