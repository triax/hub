package api

import (
	"net/http"
	"time"

	"github.com/otiai10/marmoset"
)

func AuthLogout(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "hub-identity-token",
		Value:   "",
		Path:    "/",
		Expires: time.Time{},
	})
	marmoset.Render(w).JSON(http.StatusOK, marmoset.P{"ok": true})
}
