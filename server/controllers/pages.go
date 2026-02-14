package controllers

import (
	"net/http"

	"github.com/otiai10/marmoset"
)

// serveSPA は全ページルートで index.html (SPA) を配信する共通ハンドラ。
// Vite SPA のクライアントサイドルーティングにより、ページの描画はフロント側で行われる。
func serveSPA(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("index", marmoset.P{})
}

var (
	Members     = http.HandlerFunc(serveSPA)
	Member      = http.HandlerFunc(serveSPA)
	Events      = http.HandlerFunc(serveSPA)
	Event       = http.HandlerFunc(serveSPA)
	Equips      = http.HandlerFunc(serveSPA)
	Equip       = http.HandlerFunc(serveSPA)
	EquipCreate = http.HandlerFunc(serveSPA)
	EquipReport = http.HandlerFunc(serveSPA)
	EquipEdit   = http.HandlerFunc(serveSPA)
	Uniforms    = http.HandlerFunc(serveSPA)
)
