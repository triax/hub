package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type (
	// SessionUserClaims はセッション Cookie の JWT に載せるクレーム。
	// RegisteredClaims は値で埋め込む。ポインタ埋め込みだと、登録済みクレームを
	// 1 つも持たないトークンの検証で nil 参照になる。
	SessionUserClaims struct {
		jwt.RegisteredClaims
		SlackID string
	}
	Myself struct {
		OpenID SlackOpenIDUserInfo `json:"openid"`
		Slack  SlackUser           `json:"slack"`
		Team   SlackTeam           `json:"team"`
	}
)
