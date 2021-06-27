package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/otiai10/marmoset"
)

type (
	SlackProfile struct {
		Title string `json:"title"`
		// 名前
		RealName              string `json:"real_name"`
		RealNameNormalized    string `json:"real_name_normalized"`
		DisplayName           string `json:"display_name"`
		DisplayNameNormalized string `json:"display_name_normalized"`
		FirstName             string `json:"first_name"`
		LastName              string `json:"last_name"`
		// アイコン
		AvatarHash    string `json:"avatar_hash"`
		IsCustomImage bool   `json:"is_custom_image"`
		ImageOriginal string `json:"image_original"`
		Image512      string `json:"image_512"`
		Team          string `json:"team"`
	}
	SlackMember struct {
		ID                string       `json:"id"`
		TeamID            string       `json:"team_id"`
		Name              string       `json:"name"`
		RealName          string       `json:"real_name"`
		Deleted           bool         `json:"deleted"`
		Color             string       `json:"color"`
		Profile           SlackProfile `json:"profile"`
		IsAdmin           bool         `json:"is_admin"`
		IsOwner           bool         `json:"is_owner"`
		IsPrimaryOwner    bool         `json:"is_primary_owner"`
		IsRestricted      bool         `json:"is_restricted"`
		IsUltraRestricted bool         `json:"is_ultra_restricted"`
		IsBot             bool         `json:"is_bot"`
		IsAppUser         bool         `json:"is_app_user"`
		IsEmailConfirmed  bool         `json:"is_email_confirmed"`
		Updated           int          `json:"updated"`
	}
	SlackMembersResponse struct {
		OK       bool          `json:"ok"`
		Error    string        `json:"error"`
		Members  []SlackMember `json:"members"`
		CacheTS  int           `json:"cache_ts"`
		Metadata struct {
			NextCursor string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
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

	slackres := new(SlackMembersResponse)
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

	marmoset.RenderJSON(w, http.StatusOK, marmoset.P{
		"count": len(slackres.Members),
	})
}
