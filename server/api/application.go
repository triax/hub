package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/models"
)

const slackChannelApplications = "C06SZGR7L1W" // #入部退部者処理

func isApplicationAdmin(ctx context.Context, slackID string) (bool, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return false, err
	}
	defer client.Close()
	member := models.Member{}
	key := datastore.NameKey(models.KindMember, slackID, nil)
	if err := client.Get(ctx, key, &member); err != nil && !models.IsFiledMismatch(err) {
		return false, err
	}
	return member.Slack.IsAdmin, nil
}

func CreateApplication(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)

	var input struct {
		Type            string            `json:"type"`
		Email           string            `json:"email"`
		Name            string            `json:"name"`
		Fields          map[string]string `json:"fields"`
		ConsentAgreedAt *time.Time        `json:"consent_agreed_at"`
	}
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if input.Type == "" || input.Email == "" || input.Name == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "type, email, name are required"})
		return
	}
	if input.ConsentAgreedAt == nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "consent_agreed_at is required"})
		return
	}

	app := &models.Application{
		Type:            input.Type,
		Email:           input.Email,
		Name:            input.Name,
		Fields:          input.Fields,
		ConsentAgreedAt: *input.ConsentAgreedAt,
		CreatedAt:       time.Now(),
	}

	if input.Type == "onboarding" {
		app.Steps = []models.ApplicationStep{
			{Key: "slack_invited", Label: "Slack 招待", Done: false},
			{Key: "google_groups_added", Label: "Google Groups 追加", Done: false},
			{Key: "hudl_added", Label: "Hudl 追加", Done: false},
		}
	}

	id := uuid.NewString()
	if err := models.PutApplication(req.Context(), id, app); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	if input.Type == "onboarding" {
		go func() {
			api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
			msg := fmt.Sprintf("<!channel> 新しい入部申請が届きました。\nhttps://hub.triax.football/applications")
			if _, _, err := api.PostMessage(slackChannelApplications, slack.MsgOptionText(msg, false)); err != nil {
				log.Printf("[ERROR] 9010 Slack notification for new application: %v", err)
			}
		}()
	}

	type response struct {
		ID string `json:"id"`
		*models.Application
	}
	render.JSON(http.StatusCreated, response{ID: id, Application: app})
}

func GetApplications(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)

	callerID := filters.GetSessionUserContext(req)
	ok, err := isApplicationAdmin(req.Context(), callerID)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	appType := req.URL.Query().Get("type")
	apps, ids, err := models.ListApplications(req.Context(), appType)
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	type entry struct {
		ID string `json:"id"`
		*models.Application
	}
	result := make([]entry, len(apps))
	for i, a := range apps {
		result[i] = entry{ID: ids[i], Application: a}
	}
	render.JSON(http.StatusOK, marmoset.P{"applications": result})
}

func UpdateApplication(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	id := chi.URLParam(req, "id")

	callerID := filters.GetSessionUserContext(req)
	ok, err := isApplicationAdmin(req.Context(), callerID)
	if err != nil || !ok {
		render.JSON(http.StatusForbidden, marmoset.P{"error": "forbidden"})
		return
	}

	app, err := models.GetApplication(req.Context(), id)
	if err != nil || app == nil {
		render.JSON(http.StatusNotFound, marmoset.P{"error": "not found"})
		return
	}

	var input struct {
		Steps []models.ApplicationStep `json:"steps,omitempty"`
		Done  *bool                    `json:"done,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	if input.Steps != nil {
		app.Steps = input.Steps
	}
	if input.Done != nil {
		app.Done = *input.Done
	}

	if err := models.PutApplication(req.Context(), id, app); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	type response struct {
		ID string `json:"id"`
		*models.Application
	}
	render.JSON(http.StatusOK, response{ID: id, Application: app})
}
