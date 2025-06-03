package utils

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// RandomString 创建一个随机数(包含大小写)
func RandStringBytesRandomString(n int) string {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// RandomStringWithNumber 创建一个包含大小写并带数字的随机数
func RandomStringWithNumber(n int) string {
	var letters = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

func RandStringBytes(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyz"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// RandStringByNumLowercase 生成指定长度的随机字符串，包含小写字母和数字
func RandStringByNumLowercase(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz0123456789"
	// 创建一个本地的随机数生成器
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Intn(len(letterBytes))]
	}
	return string(b)
}

var rfc1123Pattern = regexp.MustCompile(`[^a-z0-9-]+`)

func ToRFC1123Name(s string) string {
	s = strings.ToLower(s)
	s = rfc1123Pattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) == 0 {
		return "port"
	}
	return s
}

func RandRFC1123Suffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
