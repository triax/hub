package controllers

import (
	"net/http"
	"time"

	m "github.com/otiai10/marmoset"
	"github.com/triax/hub/server"
)

// ログイン後に来るトップページ
func Top(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("index", nil)
}

// ログインシーケンスを始めるためのランディングページにすぎない
func Login(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("login", nil)
}

func Logout(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:  server.SessionCookieName,
		Value: "", Path: "/", Expires: time.Unix(0, 0),
		MaxAge: -1, HttpOnly: true,
	})
	http.Redirect(w, req, "/login?error=logout", http.StatusTemporaryRedirect)
}

func NotFound(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("404", nil)
}

func ErrorsPage(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("errors", nil)
}
