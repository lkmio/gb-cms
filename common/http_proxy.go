package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func HttpForwardTo(path string, w http.ResponseWriter, r *http.Request) {
	target, _ := url.Parse(fmt.Sprintf("%s%s", Config.MediaServer, path))
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))

			// 复制所有原始请求头
			for name, values := range r.Header {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}

			// 保留原始查询参数
			req.URL.RawQuery = r.URL.RawQuery

			// 复制请求体（如果有）
			if r.Body != nil {
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Error reading request body", http.StatusInternalServerError)
					return
				}
				r.Body.Close()
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
				req.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(bodyBytes)), nil
				}
			}

			// 复制其他请求属性
			req.Method = r.Method
			req.Proto = r.Proto
			req.ProtoMajor = r.ProtoMajor
			req.ProtoMinor = r.ProtoMinor
		},
	}

	proxy.ServeHTTP(w, r)
}
