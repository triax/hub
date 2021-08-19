package models

import (
	"github.com/golang-jwt/jwt"
)

type (
	SessionUserClaims struct {
		*jwt.StandardClaims
		Info SlackOpenIDUserInfo
	}
)
