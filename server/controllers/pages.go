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

func Events(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("events", marmoset.P{})
}

func Event(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("events/[id]", marmoset.P{})
}

func Equips(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("equips", marmoset.P{})
}

func Equip(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("equips/[id]", marmoset.P{})
}

func EquipCreate(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("equips/create", marmoset.P{})
}

func EquipReport(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	render.HTML("equips/report", marmoset.P{})
}
