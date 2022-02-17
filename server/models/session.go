package models

import (
	"github.com/golang-jwt/jwt"
)

type (
	SessionUserClaims struct {
		*jwt.StandardClaims
		SlackID string
	}
	Myself struct {
		OpenID SlackOpenIDUserInfo `json:"openid"`
		Slack  SlackUser           `json:"slack"`
		Team   SlackTeam           `json:"team"`
	}
)
