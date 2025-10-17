package api

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"os"
	"time"
)

var (
	AdminMD5    string // 明文密码"admin"的MD5值
	PwdMD5      string
	StartUpTime time.Time
)

func GenerateTempPwd() string {
	// 根据字母数字符号生成12位随机密码
	// 字母数字符号
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	// 随机数
	rand.Seed(time.Now().UnixNano())
	// 生成12位随机密码
	b := make([]byte, 12)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

// ReadTempPwd 读取临时密码
func ReadTempPwd() (plaintext string, md5Hex string) {
	// 从文件中读取密码
	pwd, err := os.ReadFile("./data/pwd.txt")
	if err != nil {
		// 生成密码
		plaintext = "admin"

		// 计算md5
		hash := md5.Sum([]byte(plaintext))
		pwd = []byte(hex.EncodeToString(hash[:]))

		// 写入文件
		err = os.WriteFile("./data/pwd.txt", pwd, 0644)
		if err != nil {
			panic(err)
		}
	}

	md5Hex = string(pwd)
	return
}
