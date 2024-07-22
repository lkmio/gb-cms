package main

import (
	"bytes"
	"encoding/xml"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"io/ioutil"
	"strings"
)

func GbkToUtf8(s []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())

	d, e := ioutil.ReadAll(reader)

	if e != nil {

		return nil, e
	}

	return d, nil
}

func DoDecodeXML(data []byte, message interface{}) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(c string, i io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(c, i)
	}

	return decoder.Decode(message)
}

func DecodeXML(data []byte, message interface{}) error {
	uft8Data := []byte(strings.Replace(string(data), "GB2312", "UTF-8", 1))

	err := DoDecodeXML(uft8Data, message)
	if err != nil {
		utf8, err := GbkToUtf8(uft8Data)
		if err != nil {
			return err
		}

		err = DoDecodeXML(uft8Data, utf8)
	}

	return err
}
