package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"strings"
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

func GetRootElementName(data string) string {
	reader := strings.NewReader(data)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() && scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		return line[1 : len(line)-1]
	}

	return ""
}

func GetCmdType(data string) string {
	startIndex := strings.Index(data, CmdTagStart)
	endIndex := strings.Index(data, CmdTagEnd)
	if startIndex <= 0 || endIndex <= 0 || endIndex+len(CmdTagStart) <= startIndex {
		return ""
	}

	return data[startIndex+len(CmdTagStart) : endIndex]
}
