package api

import (
	"net/http"
	"time"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server"
)

func AuthLogout(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:  server.SessionCookieName,
		Value: "", Path: "/", Expires: time.Unix(0, 0),
		MaxAge: -1, HttpOnly: true,
	})
	marmoset.Render(w).JSON(http.StatusOK, marmoset.P{"ok": true})
}
