package slackbot

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type SlackAPI interface {
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
}

type Bot struct {
	VerificationToken string
	SlackAPI          SlackAPI
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

	switch payload.Type {
	case slackevents.URLVerification:
		bot.onURLVerification(req, w, payload)
	case slackevents.AppMention:
		bot.onMention(req, w, payload)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (bot Bot) onURLVerification(req *http.Request, w http.ResponseWriter, payload Payload) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload.Challenge))
}

func (bot Bot) onMention(req *http.Request, w http.ResponseWriter, payload Payload) {
	a, b, err := bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText("Hello!", false))
	log.Println(a, b, err)
}
