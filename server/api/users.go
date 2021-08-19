package api

import (
	"net/http"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
)

func GetCurrentUser(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	info := filters.GetSessionUserContext(req)
	if info == nil {
		render.JSON(http.StatusBadRequest, marmoset.P{})
	}
	render.JSON(http.StatusOK, info)
}
