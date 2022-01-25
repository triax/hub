package slackbot

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/otiai10/largo"
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

	switch {
	case payload.Type == slackevents.URLVerification:
		bot.onURLVerification(req, w, payload)
	case payload.Event.Type == slackevents.AppMention:
		bot.onMention(req, w, payload)
	default:
		log.Printf("UNKNOWN EVENT TYPE: %+v\n", payload)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (bot Bot) onURLVerification(req *http.Request, w http.ResponseWriter, payload Payload) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload.Challenge))
}

func (bot Bot) onMention(req *http.Request, w http.ResponseWriter, payload Payload) {
	tokens := largo.Tokenize(payload.Event.Text)[1:]
	opts := []slack.MsgOption{slack.MsgOptionText("You said: "+strings.Join(tokens, " "), false)}
	if payload.Event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(payload.Event.ThreadTimeStamp))
	}
	a, b, err := bot.SlackAPI.PostMessage(payload.Event.Channel, opts...)
	log.Println(a, b, err)
}
