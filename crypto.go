package kuu

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

// MD5 加密
func MD5(p string, upper ...bool) (v string) {
	h := md5.New()
	h.Write([]byte(p))
	v = hex.EncodeToString(h.Sum(nil))
	if len(upper) > 0 && upper[0] {
		v = strings.ToUpper(v)
	}
	return
}

// Sha1 加密
func Sha1(p string, upper ...bool) (v string) {
	d := sha1.New()
	d.Write([]byte(p))
	v = hex.EncodeToString(d.Sum([]byte(nil)))
	if len(upper) > 0 && upper[0] {
		v = strings.ToUpper(v)
	}
	return
}

// GenerateFromPassword 生成新密码
func GenerateFromPassword(p string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(hash), err
}

// CompareHashAndPassword 密码比对
func CompareHashAndPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err
}

// Base64Encode Base64编码
func Base64Encode(p string) (v string) {
	v = base64.StdEncoding.EncodeToString([]byte(p))
	return
}

// Base64Decode Base64解码
func Base64Decode(p string) (v string) {
	decoded, err := base64.StdEncoding.DecodeString(p)
	if err != nil {
		ERROR(err)
	}
	v = string(decoded)
	return
}

// Base64URLEncode Base64 URL编码
func Base64URLEncode(p string) (v string) {
	v = base64.URLEncoding.EncodeToString([]byte(p))
	return
}

// Base64URLDecode Base64 URL解码
func Base64URLDecode(p string) (v string) {
	decoded, err := base64.URLEncoding.DecodeString(p)
	if err != nil {
		ERROR(err)
	}
	v = string(decoded)
	return
}
