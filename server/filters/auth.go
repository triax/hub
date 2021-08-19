package filters

import (
	"context"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

type (
	AuthFilter struct {
		marmoset.Filter
	}
	ContextKey string
)

const (
	SessionContextKey ContextKey = "session_user"
)

func SetSessionUserContext(req *http.Request, info *models.SlackOpenIDUserInfo) *http.Request {
	ctx := context.WithValue(req.Context(), SessionContextKey, info)
	return req.WithContext(ctx)
}

func GetSessionUserContext(req *http.Request) *models.SlackOpenIDUserInfo {
	return req.Context().Value(SessionContextKey).(*models.SlackOpenIDUserInfo)
}

// TODO: Basic authやめる
func (auth *AuthFilter) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	cookie, err := req.Cookie("hub-identity-token")
	if err != nil {
		http.Redirect(w, req, "/login?error="+err.Error(), http.StatusTemporaryRedirect)
		return
	}

	claims := new(models.SessionUserClaims)
	if _, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
	}); err != nil {
		http.Redirect(w, req, "/login?error="+err.Error(), http.StatusTemporaryRedirect)
		return
	}

	auth.Next.ServeHTTP(w, SetSessionUserContext(req, &claims.Info))
}
