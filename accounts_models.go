package kuu

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
)

// SignHistory
type SignHistory struct {
	gorm.Model `rest:"*"`
	Request    string
	SecretID   uint
	SecretData string
	Token      string
	Method     string
}

// SignSecret
type SignSecret struct {
	gorm.Model `rest:"*"`
	UID        uint
	Secret     string
	Token      string
	Iat        int64
	Exp        int64
	Method     string
}

// SignContext
type SignContext struct {
	Token   string
	UID     uint
	Payload jwt.MapClaims
	Secret  *SignSecret
}

// IsValid
func (s *SignContext) IsValid() (ret bool) {
	if err := s.Payload.Valid(); err == nil && s.Token != "" && s.UID != 0 {
		ret = true
	}
	return
}
