package controllers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

func RedirectConditioningForm(w http.ResponseWriter, req *http.Request) {

	id := filters.GetSessionUserContext(req)
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 4001, err.Error()), http.StatusTemporaryRedirect)
		return
	}
	defer client.Close()

	myself := models.Member{}
	if err := client.Get(
		ctx, datastore.NameKey(models.KindMember, id, nil), &myself,
	); err != nil && !models.IsFiledMismatch(err) {
		http.Redirect(w, req, fmt.Sprintf("/errors?code=%d&error=%s", 4002, err.Error()), http.StatusTemporaryRedirect)
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
