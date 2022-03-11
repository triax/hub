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
	// 使うAPIだけ追加する
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	GetReactions(item slack.ItemRef, params slack.GetReactionsParameters) ([]slack.ItemReaction, error)
	GetConversationReplies(params *slack.GetConversationRepliesParameters) (msgs []slack.Message, hasMore bool, nextCursor string, err error)
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

	w.WriteHeader(http.StatusAccepted)

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
	if len(tokens) == 0 {
		return
	}
	switch tokens[0] {
	case "既読", "既読チェック", "react", "reaction": // 既読チェック
		bot.onMentionReadCheck(req, w, payload)
	default:
		bot.echo(tokens, payload)
	}
}

func (bot Bot) echo(tokens []string, payload Payload) {
	opts := []slack.MsgOption{slack.MsgOptionText("You said: "+strings.Join(tokens, " "), false)}
	if payload.Event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(payload.Event.ThreadTimeStamp))
	}
	a, b, err := bot.SlackAPI.PostMessage(payload.Event.Channel, opts...)
	log.Println("[echo]", a, b, err)
}

func (bot Bot) onMentionReadCheck(req *http.Request, w http.ResponseWriter, payload Payload) {
	if payload.Event.ThreadTimeStamp == "" {
		bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText("スレッドにおいて有効です", false))
		return
	}
	reactions, err := bot.SlackAPI.GetReactions(slack.NewRefToMessage(payload.Event.Channel, payload.Event.ThreadTimeStamp), slack.NewGetReactionsParameters())
	if err != nil {
		log.Println(err.Error())
		return
	}
	log.Printf("%+v\n", reactions)
}
