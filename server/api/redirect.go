package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

func RedirectConditioningForm(w http.ResponseWriter, req *http.Request) {

	render := marmoset.Render(w, true)
	id := filters.GetSessionUserContext(req)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	myself := models.Member{}
	if err := client.Get(
		ctx, datastore.NameKey(models.KindMember, id, nil), &myself,
	); err != nil && !models.IsFiledMismatch(err) {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	position := req.URL.Query().Get("position")
	label := req.URL.Query().Get("label")
	switch label {
	case "after":
		label = "運動後"
	default:
		label = "運動前"
	}
	link := fmt.Sprintf(
		server.HubConditioningCheckSheetURL(),
		myself.Slack.ID,
		myself.Slack.Profile.DisplayName,
		strings.ToUpper(position),
		label,
	)

	http.Redirect(w, req, link, http.StatusTemporaryRedirect)

	ts := req.URL.Query().Get("slack_timestamp")
	ch := req.URL.Query().Get("response_channel")
	api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
	api.PostMessage(ch, slack.MsgOptionText(":white_check_mark: "+myself.Slack.Profile.DisplayName, false), slack.MsgOptionTS(ts))
}
