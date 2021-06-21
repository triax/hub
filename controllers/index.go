package controllers

import (
	"net/http"

	"github.com/otiai10/marmoset"
)

func Index(w http.ResponseWriter, r *http.Request) {
	marmoset.Render(w).HTML("index.html", marmoset.P{
		"name": "otiai10",
	})
}
