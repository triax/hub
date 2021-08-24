package api

import (
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

func GetCurrentUser(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	info := filters.GetSessionUserContext(req)
	if info == nil {
		render.JSON(http.StatusBadRequest, marmoset.P{})
	}

	render.JSON(http.StatusOK, info)
}

func ListMembers(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	members := []models.Member{}
	query := datastore.NewQuery(models.KindMember)
	if req.URL.Query().Get("include_deleted") != "1" {
		query = query.Filter("Slack.Deleted =", false)
	}
	if _, err := client.GetAll(ctx, query, &members); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, members)
}

func GetMember(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	id := req.FormValue("id")
	member := models.Member{}
	key := datastore.NameKey(models.KindMember, id, nil)
	if err := client.Get(ctx, key, &member); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, member)
}
