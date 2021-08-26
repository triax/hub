package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
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
	defer client.Close()

	events := []models.Event{}
	query := datastore.NewQuery(models.KindEvent).
		Filter("Google.StartTime >", time.Now().Unix()*1000).
		Order("Google.StartTime")

	if _, err := client.GetAll(ctx, query, &events); err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusOK, events)
}

func AnswerEvent(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)
	myself := filters.GetSessionUserContext(req)

	body := struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
		Type   models.ParticipationType `json:"type"`
		Params map[string]interface{}   `json:"params"`
	}{}
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		render.JSON(http.StatusInternalServerError, marmoset.P{"error": err.Error()})
		return
	}
	defer client.Close()

	member := models.Member{}
	if err := client.Get(ctx, datastore.NameKey(models.KindMember, myself.Sub, nil), &member); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	event := models.Event{}
	if _, err := client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		key := datastore.NameKey(models.KindEvent, body.Event.ID, nil)
		if err := tx.Get(key, &event); err != nil {
			return err
		}
		if event.ParticipationsJSONString == "" {
			event.ParticipationsJSONString = "{}"
		}
		parts := models.Participations{}
		if err := json.NewDecoder(strings.NewReader(event.ParticipationsJSONString)).Decode(&parts); err != nil {
			return err
		}
		parts[myself.Sub] = models.Participation{
			Type:    body.Type,
			Name:    member.Slack.Profile.RealName,
			Picture: member.Slack.Profile.Image512,
			Params:  body.Params,
		}
		b, err := json.Marshal(parts)
		if err != nil {
			return err
		}
		event.ParticipationsJSONString = string(b)
		if _, err := tx.Put(key, &event); err != nil {
			return err
		}
		return nil
	}); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}

	render.JSON(http.StatusAccepted, event)
}
