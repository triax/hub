package api

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

func ListMembers(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	members := []models.Member{}
	query := datastore.NewQuery(models.KindMember)
	if req.URL.Query().Get("include_deleted") != "1" {
		query = query.Filter("Slack.Deleted =", false)
	}
	if _, err := client.GetAll(ctx, query, &members); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	if req.URL.Query().Get("cached") == "1" {
		age := 4 * 60 * 60 // 4時間
		w.Header().Add("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", age))
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
	defer client.Close()

	id := chi.URLParam(req, "id")
	member := models.Member{}
	key := datastore.NameKey(models.KindMember, id, nil)
	if err := client.Get(ctx, key, &member); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, member)
}
