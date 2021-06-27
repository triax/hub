package controllers

import (
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/models"
)

func Index(w http.ResponseWriter, r *http.Request) {
	marmoset.Render(w).HTML("index.html", marmoset.P{
		"active": "home",
		"name":   "otiai10",
	})
}

func Members(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		marmoset.RenderJSON(w, http.StatusInternalServerError, marmoset.P{
			"code":    5001,
			"message": err.Error(),
		})
		return
	}
	members := []models.Member{}
	query := datastore.NewQuery(models.KindMember)
	if _, err := client.GetAll(ctx, query, &members); err != nil {
		marmoset.RenderJSON(w, http.StatusInternalServerError, marmoset.P{
			"code":    5002,
			"message": err.Error(),
		})
		return
	}

	fmt.Printf("%+v\n", members)

	marmoset.Render(w).HTML("members.html", marmoset.P{
		"active":  "members",
		"members": members,
	})
}
