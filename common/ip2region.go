package common

import (
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"strings"
)

var (
	searcher *xdb.Searcher
)

func LoadIP2RegionDB(path string) error {
	// 1、从 dbPath 加载整个 xdb 到内存
	cBuff, err := xdb.LoadContentFromFile(path)
	if err != nil {
		return err
	}

	// 2、用全局的 cBuff 创建完全基于内存的查询对象。
	searcher, err = xdb.NewWithBuffer(xdb.IPv4, cBuff)
	if err != nil {
		return err
	}

	return nil
}

func IP2Region(ip string) (string, error) {
	if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "192.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || strings.HasPrefix(ip, "::1") {
		return "内网IP", nil
	}
	// 3、查询
	region, err := searcher.SearchByStr(ip)
	if err != nil {
		return "", err
	}

	// 合并成一个地址
	var addressList []string
	for _, address := range strings.Split(region, "|") {
		if address == "" || address == "中国" || address == "0" {
			continue
		}

		var same bool
		for _, s := range addressList {
			if s == address {
				same = true
				break
			}
		}

		if same {
			continue
		}

		addressList = append(addressList, address)
	}

	if length := len(addressList); length == 0 {
		return "", nil
	} else if length > 1 {
		// 最后一个地址空格分开
		addressList[length-2] += " "
	}

	return strings.Join(addressList, ""), nil
}
