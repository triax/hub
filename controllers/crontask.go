package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/models"
)

func CronFetchSlackMembers(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	endpoint := "https://slack.com/api/users.list"

	slackreq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		fmt.Println("[ERROR]", 4001, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token := os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN")
	slackreq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	res, err := http.DefaultClient.Do(slackreq)
	if err != nil || res.StatusCode != http.StatusOK {
		fmt.Println("[ERROR]", 4002, err, res.StatusCode, res.Status)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slackres := new(models.SlackMembersResponse)
	if err := json.NewDecoder(res.Body).Decode(slackres); err != nil {
		fmt.Println("[ERROR]", 4003, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !slackres.OK {
		fmt.Println("[ERROR]", 4004, slackres.Error)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		fmt.Println("[ERROR]", 4005, slackres.Error)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	keys := []*datastore.Key{}
	members := []models.Member{}
	for _, m := range slackres.Members {
		if m.IsBot || m.IsAppUser {
			continue
		}
		keys = append(keys, datastore.NameKey("Member", m.ID, nil))
		members = append(members, models.Member{Slack: m})
	}

	if _, err := client.PutMulti(ctx, keys, members); err != nil {
		fmt.Println("[ERROR]", 4006, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	marmoset.RenderJSON(w, http.StatusOK, marmoset.P{
		"message": "ok",
		"count":   len(members),
	})
}
