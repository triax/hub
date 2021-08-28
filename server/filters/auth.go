package filters

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

type (
	AuthFilter struct {
		marmoset.Filter
		API      bool
		LocalDev bool
	}
	ContextKey string
)

const (
	SessionContextKey ContextKey = "session_user"
)

func SetSessionUserContext(req *http.Request, myself *models.Myself) *http.Request {
	ctx := context.WithValue(req.Context(), SessionContextKey, myself)
	return req.WithContext(ctx)
}

func GetSessionUserContext(req *http.Request) *models.Myself {
	return req.Context().Value(SessionContextKey).(*models.Myself)
}

func (auth *AuthFilter) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if auth.API && auth.LocalDev {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	}

	if auth.LocalDev {
		f, _ := os.Open("server/filters/local-user.json")
		myself := models.Myself{}
		json.NewDecoder(f).Decode(&myself)
		f.Close()
		auth.Next.ServeHTTP(w, SetSessionUserContext(req, &myself))
		return
	}

	cookie, err := req.Cookie("hub-identity-token")
	if err != nil {
		if auth.API {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			http.Redirect(w, req, "/login", http.StatusTemporaryRedirect)
		}
		return
	}

	claims := new(models.SessionUserClaims)
	if _, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
	}); err != nil {
		if auth.API {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			http.Redirect(w, req, "/login?error="+err.Error(), http.StatusTemporaryRedirect)
		}
		return
	}
	if claims.Myself.OpenID.Sub == "" || claims.Myself.OpenID.Picture == "" {
		http.Redirect(w, req, "/login?error=reset", http.StatusTemporaryRedirect)
		return
	}

	auth.Next.ServeHTTP(w, SetSessionUserContext(req, &claims.Myself))
}
