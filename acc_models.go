package kuu

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"gopkg.in/guregu/null.v3"
)

// SignHistory
type SignHistory struct {
	gorm.Model `rest:"*" displayName:"登录历史"`
	SecretID   uint   `name:"密钥ID"`
	SecretData string `name:"密钥"`
	Token      string `name:"令牌" gorm:"size:4096"`
	Method     string `name:"登录/登出"`
}

// SignSecret
type SignSecret struct {
	gorm.Model `rest:"*" displayName:"令牌密钥"`
	UID        uint      `name:"用户ID"`
	SubDocID   uint      `name:"扩展档案ID"`
	Desc       string    `name:"令牌描述"`
	Secret     string    `name:"令牌密钥"`
	Token      string    `name:"令牌" gorm:"size:4096"`
	Iat        int64     `name:"令牌签发时间戳"`
	Exp        int64     `name:"令牌过期时间戳"`
	Method     string    `name:"登录/登出"`
	IsAPIKey   null.Bool `name:"是否API Key"`
	Type       string    `name:"令牌类型"`
}

// SignContext
type SignContext struct {
	Token    string
	Type     string
	Lang     string
	UID      uint
	SubDocID uint
	Payload  jwt.MapClaims
	Secret   *SignSecret
}

// IsValid
func (s *SignContext) IsValid() (ret bool) {
	if s == nil {
		return
	}
	if err := s.Payload.Valid(); err == nil && s.Token != "" && s.UID != 0 && s.Secret != nil {
		ret = true
	}
	return
}
