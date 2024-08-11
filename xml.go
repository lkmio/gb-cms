package main

import (
	"bytes"
	"encoding/xml"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
)

func GbkToUtf8(s []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	return io.ReadAll(reader)
}

func DoDecodeXML(data []byte, message interface{}) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(c string, i io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(c, i)
	}

	return decoder.Decode(message)
}

func DecodeXML(data []byte, message interface{}) error {
	//uft8Data := []byte(strings.Replace(string(data), "GB2312", "UTF-8", 1))
	uft8Data := data
	err := DoDecodeXML(uft8Data, message)
	if err != nil {
		uft8Data, err = GbkToUtf8(uft8Data)
		if err != nil {
			return err
		}

		err = DoDecodeXML(uft8Data, message)
	}

	return err
}
