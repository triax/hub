package controllers

import (
	"net/http"

	"github.com/otiai10/marmoset"
)

func Members(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("members", marmoset.P{})
}

func Member(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("members/[id]", marmoset.P{})
}
