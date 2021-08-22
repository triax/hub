package api

import (
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/models"
)

func ListEvents(w http.ResponseWriter, req *http.Request) {

	render := marmoset.Render(w)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	events := []models.Event{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindEvent), &events); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, events)
}
