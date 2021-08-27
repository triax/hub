package models

import (
	"github.com/golang-jwt/jwt"
)

type (
	SessionUserClaims struct {
		*jwt.StandardClaims
		Myself Myself
	}
	Myself struct {
		OpenID SlackOpenIDUserInfo `json:"openid"`
		Slack  SlackMember         `json:"slack"`
	}
)
