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

	slackID := filters.GetSessionUserContext(req)
	if slackID == "" {
		render.JSON(http.StatusForbidden, marmoset.P{})
	}

	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	member := models.Member{}
	key := datastore.NameKey(models.KindMember, slackID, nil)
	if err := client.Get(ctx, key, &member); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	myself := &models.Myself{Slack: member.Slack, Team: member.Team}

	// https://cloud.google.com/appengine/docs/standard/nodejs/how-requests-are-handled#response_caching
	w.Header().Add("Cache-Control", "no-store, max-age=0")

	render.JSON(http.StatusOK, myself)
}
