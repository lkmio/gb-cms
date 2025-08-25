package main

import (
	"net/http"
	"reflect"
	"strconv"
	"time"
)

// parseQueryParams 使用反射解析 URL 查询参数并填充到结构体中
func parseQueryParams(c func(key string) string, v interface{}) (interface{}, error) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			// 如果指针为 nil，创建一个新的实例
			val = reflect.New(val.Type().Elem())
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, nil
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// 获取字段名
		fieldName := field.Tag.Get("json")
		if fieldName == "" {
			fieldName = field.Name
		}

		// 从 URL 参数中获取值
		value := c(fieldName)
		if value == "" {
			continue
		}

		// 根据字段类型设置值
		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(value)
		case reflect.Int:
			intValue, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			fieldValue.SetInt(int64(intValue))
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return nil, err
			}
			fieldValue.SetBool(boolValue)
		}
	}

	return val.Addr().Interface(), nil
}

func withJsonParams[T any](f func(params T, w http.ResponseWriter, req *http.Request), params T) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		newParams := new(T)
		if err := HttpDecodeJSONBody(w, req, newParams); err != nil {
			Sugar.Errorf("解析请求体失败 err: %s path: %s", err.Error(), req.URL.Path)
			_ = httpResponseError(w, err.Error())
			return
		}

		f(*newParams, w, req)
	}
}

func withJsonResponse[T any](f func(params T, w http.ResponseWriter, req *http.Request) (interface{}, error), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		newParams := new(T)
		if err := HttpDecodeJSONBody(w, req, newParams); err != nil {
			Sugar.Errorf("解析请求体失败 err: %s path: %s", err.Error(), req.URL.Path)
			_ = httpResponseError(w, err.Error())
			return
		}

		responseBody, err := f(*newParams, w, req)
		if err != nil {
			_ = httpResponseError(w, err.Error())
		} else if responseBody != nil {
			_ = httpResponseOK(w, responseBody)
		}
	}
}

func withJsonResponse2(f func(w http.ResponseWriter, req *http.Request) (interface{}, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		responseBody, err := f(w, req)
		if err != nil {
			_ = httpResponseError(w, err.Error())
		} else if responseBody != nil {
			_ = httpResponseJson(w, responseBody)
		}
	}
}

func withQueryStringParams[T any](f func(params T, w http.ResponseWriter, req *http.Request) (interface{}, error), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var newParams T
		query := req.URL.Query()
		result, err := parseQueryParams(func(key string) string {
			if key == "token" {
				cookie, err := req.Cookie("token")
				if err != nil {
					panic(err)
				}
				return cookie.Value
			}

			return query.Get(key)
		}, newParams)
		if err != nil {
			_ = httpResponseError(w, err.Error())
			return
		}

		responseBody, err := f(result.(T), w, req)
		if err != nil {
			_ = httpResponseError(w, err.Error())
		} else if responseBody != nil {
			_ = httpResponseJson(w, responseBody)
		}
	}
}

func withFormDataParams[T any](f func(params T, w http.ResponseWriter, req *http.Request) (interface{}, error), params interface{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		var newParams T
		result, err := parseQueryParams(func(key string) string {
			if key == "token" {
				cookie, err := req.Cookie("token")
				if err != nil {
					panic(err)
				}
				return cookie.Value
			}

			return req.FormValue(key)
		}, newParams)
		if err != nil {
			_ = httpResponseError(w, err.Error())
			return
		}

		responseBody, err := f(result.(T), w, req)
		if err != nil {
			_ = httpResponseError(w, err.Error())
		} else if responseBody != nil {
			_ = httpResponseJson(w, responseBody)
		}
	}
}

// 验证和刷新token
func withVerify(f func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ok := TokenManager.Refresh(cookie.Value, time.Now())
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		f(w, req)
	}
}

func withVerify2(onSuccess func(w http.ResponseWriter, req *http.Request), onFailure func(w http.ResponseWriter, req *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("token")
		if err == nil && TokenManager.Refresh(cookie.Value, time.Now()) {
			onSuccess(w, req)
		} else {
			onFailure(w, req)
		}
	}
}
