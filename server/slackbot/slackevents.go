package slackbot

import (
	"encoding/json"
	"net/http"

	"github.com/slack-go/slack/slackevents"
)

type Bot struct {
	VerificationToken string
}

type (
	Payload struct {
		slackevents.EventsAPIEvent
		slackevents.ChallengeResponse
		Event slackevents.AppMentionEvent
	}
)

func (bot Bot) Webhook(w http.ResponseWriter, req *http.Request) {

	payload := Payload{}
	defer req.Body.Close()

	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if payload.Token != bot.VerificationToken {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if payload.Type == slackevents.URLVerification {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(payload.Challenge))
		return
	}

	// TODO
	w.WriteHeader(http.StatusOK)
}

// func (bot Bot) onBotMention(req *http.Request, w http.ResponseWriter) {

// }
