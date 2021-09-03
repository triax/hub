package controllers

import (
	"net/http"

	m "github.com/otiai10/marmoset"
)

// ログイン後に来るトップページ
func Top(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("index", nil)
}

// ログインシーケンスを始めるためのランディングページにすぎない
func Login(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("login", nil)
}

func NotFound(w http.ResponseWriter, req *http.Request) {
	m.Render(w).HTML("404", nil)
}
