package filters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

type (
	ContextKey string
)

const (
	SessionContextKey ContextKey = "session_user"
)

func SetSessionUserContext(req *http.Request, slackID string) *http.Request {
	ctx := context.WithValue(req.Context(), SessionContextKey, slackID)
	return req.WithContext(ctx)
}

func GetSessionUserContext(req *http.Request) string {
	return req.Context().Value(SessionContextKey).(string)
}

type (
	Auth struct {
		API      bool
		LocalDev bool
	}
)

func (auth *Auth) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		if auth.API && auth.LocalDev {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		}

		if auth.LocalDev {
			f, err := os.Open("server/filters/local-user.json")
			if err != nil {
				panic(err)
			}
			defer f.Close()
			myself := models.Myself{}
			if err = json.NewDecoder(f).Decode(&myself); err != nil {
				panic(err)
			}
			next.ServeHTTP(w, SetSessionUserContext(req, myself.OpenID.Sub))
			return
		}

		destination := url.QueryEscape(req.URL.String())

		cookie, err := req.Cookie(server.SessionCookieName)
		if err != nil {
			if auth.API {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				http.Redirect(w, req, "/login?goto="+destination, http.StatusTemporaryRedirect)
			}
			return
		}

		claims := new(models.SessionUserClaims)
		if _, err := jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
			// JWT署名アルゴリズムを検証してアルゴリズム混同攻撃を防ぐ
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
		}); err != nil {
			if auth.API {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				http.SetCookie(w, &http.Cookie{
					Name:  server.SessionCookieName,
					Value: "", Path: "/", Expires: time.Unix(0, 0),
					MaxAge: -1, HttpOnly: true,
				})
				http.Redirect(w, req, "/login?goto="+destination, http.StatusTemporaryRedirect)
			}
			return
		}
		if claims.SlackID == "" {
			http.SetCookie(w, &http.Cookie{
				Name:  server.SessionCookieName,
				Value: "", Path: "/", Expires: time.Unix(0, 0),
				MaxAge: -1, HttpOnly: true,
			})
			http.Redirect(w, req, "/login?goto="+destination, http.StatusTemporaryRedirect)
			return
		}
		next.ServeHTTP(w, SetSessionUserContext(req, claims.SlackID))
	})
}
