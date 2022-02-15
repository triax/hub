package models

import (
	"github.com/golang-jwt/jwt"
	"github.com/slack-go/slack"
)

type (
	SessionUserClaims struct {
		*jwt.StandardClaims
		SlackID string
	}
	Myself struct {
		OpenID SlackOpenIDUserInfo `json:"openid"`
		Slack  slack.User          `json:"slack"`
		Team   slack.TeamInfo      `json:"team"`
	}
)
