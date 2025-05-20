package main

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"
)

func TestDecodeXML(t *testing.T) {
	//var catalogBodyFos *os.File
	//if catalogBodyFos == nil {
	//	file, err := os.OpenFile("./catalog.raw", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	//	if err != nil {
	//		panic(err)
	//	}
	//	catalogBodyFos = file
	//}
	//
	//bytes := make([]byte, 4)
	//binary.BigEndian.PutUint32(bytes, uint32(len(body)))
	//if _, err := catalogBodyFos.Write(bytes); err != nil {
	//	panic(err)
	//} else if _, err = catalogBodyFos.Write([]byte(body)); err != nil {
	//	panic(err)
	//}

	t.Run("save_channels", func(t *testing.T) {
		file, err := os.ReadFile("./catalog.raw")
		if err != nil {
			panic(err)
		}
		handler := EventHandler{}
		for i := 0; i < len(file); {
			size := binary.BigEndian.Uint32(file[i:])
			i += 4
			body := file[i : i+int(size)]
			i += int(size)
			catalogResponse := &CatalogResponse{}
			err = DecodeXML(body, catalogResponse)
			if err != nil {
				panic(err)
			}

			handler.OnCatalog(catalogResponse.DeviceID, catalogResponse)
		}
	})

	//str := "3c3f786d6c2076657273696f6e3d22312e30223f3e0d0a3c51756572793e0d0a3c436d64547970653e446576696365496e666f3c2f436d64547970653e0d0a3c534e3e323c2f534e3e0d0a3c44657669636549443e33343032303030303030313332303030303030313c2f44657669636549443e0d0a3c2f51756572793e0d0a"
	str := "3c3f786d6c2076657273696f6e3d22312e302220656e636f64696e673d22474232333132223f3e0d0a3c526573706f6e73653e0d0a3c436d64547970653e436174616c6f673c2f436d64547970653e0d0a3c534e3e313c2f534e3e0d0a3c44657669636549443e33343032303030303030313332303030303030313c2f44657669636549443e0d0a3c53756d4e756d3e313c2f53756d4e756d3e0d0a3c4465766963654c697374204e756d3d2231223e0d0a3c4974656d3e0d0a3c44657669636549443e33343032303030303030313331303030303030313c2f44657669636549443e0d0a3c4e616d653e47423238313831436c69656e743c2f4e616d653e0d0a3c4d616e7566616374757265723e48616958696e3c2f4d616e7566616374757265723e0d0a3c4d6f64656c3e474232383138315f416e64726f69643c2f4d6f64656c3e0d0a3c4f776e65723e4f776e65723c2f4f776e65723e0d0a3c416464726573733e416464726573733c2f416464726573733e0d0a3c506172656e74616c3e303c2f506172656e74616c3e0d0a3c506172656e7449443e33343032303030303030313332303030303030313c2f506172656e7449443e0d0a3c5361666574795761793e303c2f5361666574795761793e0d0a3c52656769737465725761793e313c2f52656769737465725761793e0d0a3c536563726563793e303c2f536563726563793e0d0a3c5374617475733e4f4e3c2f5374617475733e0d0a3c2f4974656d3e0d0a3c2f4465766963654c6973743e0d0a3c2f526573706f6e73653e0d0a"
	data, err := hex.DecodeString(str)

	response := CatalogResponse{}
	if err = DecodeXML(data, &response); err != nil {
		panic(err)
	}
}
