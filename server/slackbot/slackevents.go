package slackbot

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/triax/hub/server/models"

	"github.com/otiai10/largo"
)

type SlackAPI interface {
	// 使うAPIだけ追加する
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	GetReactions(item slack.ItemRef, params slack.GetReactionsParameters) ([]slack.ItemReaction, error)
	GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
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
	case "備品", "備品チェック": // 備品チェック
		bot.onMentionEquipCheck(req, w, payload)
	case "予報":
		bot.onMentionAmesh(req, w, payload)
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

	resp, err := bot.SlackAPI.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: payload.Event.Channel,
		Latest:    payload.Event.ThreadTimeStamp,
		Oldest:    payload.Event.ThreadTimeStamp,
		Limit:     1,
		Inclusive: true,
	})
	if err != nil {
		bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}
	if len(resp.Messages) == 0 {
		bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText("NOT FOUND", false))
		return
	}
	parent := resp.Messages[0]
	users := regexp.MustCompile("<@[a-zA-Z0-9]+>").FindAllString(parent.Text, -1)
	reactions, err := bot.SlackAPI.GetReactions(slack.NewRefToMessage(payload.Event.Channel, payload.Event.ThreadTimeStamp), slack.NewGetReactionsParameters())
	if err != nil {
		bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}
	expected := users
	for _, r := range reactions {
		for _, ru := range r.Users {
			for i, u := range users {
				if strings.Contains(u, ru) {
					users = append(users[:i], users[i+1:]...)
				}
			}
		}
	}

	buf := bytes.NewBuffer(nil)
	err = tplReadCheck.Execute(buf, map[string]interface{}{"Expected": expected, "NotReacted": users})
	if err != nil {
		bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}
	bot.SlackAPI.PostMessage(payload.Event.Channel, slack.MsgOptionText(buf.String(), false))
}

func (bot Bot) onMentionEquipCheck(req *http.Request, w http.ResponseWriter, payload Payload) {
	ctx := req.Context()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return
	}
	defer client.Close()

	equips := []models.Equip{}
	query := datastore.NewQuery(models.KindEquip)
	if _, err := client.GetAll(ctx, query, &equips); err != nil && !models.IsFiledMismatch(err) {
		return
	}

	summary := struct {
		Unmanaged  []models.Equip
		NotUpdated []models.Equip
		Since      time.Time
	}{
		Since: time.Now().AddDate(0, 0, -7),
	}

	for i, e := range equips {
		equips[i].ID = e.Key.ID
		// 最新のHistoryだけ収集する
		query := datastore.NewQuery(models.KindCustody).Ancestor(e.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
		// Summarizeする
		if len(equips[i].History) == 0 {
			summary.Unmanaged = append(summary.Unmanaged, equips[i])
		} else if !equips[i].HasBeenUpdatedSince(summary.Since) {
			if equips[i].ForPractice {
				summary.NotUpdated = append(summary.NotUpdated, equips[i])
			}
		}
	}

	buf := bytes.NewBuffer(nil)
	if err := tplEquipsManagementSummary.Execute(buf, summary); err != nil {
		log.Println(err.Error())
		return
	}

	opts := []slack.MsgOption{slack.MsgOptionText(buf.String(), false)}
	if payload.Event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(payload.Event.ThreadTimeStamp))
	}
	_, _, err = bot.SlackAPI.PostMessage(payload.Event.Channel, opts...)
	log.Printf("[equip] %+v %v", summary, err)
}

func (bot Bot) onMentionAmesh(req *http.Request, w http.ResponseWriter, payload Payload) {
	// U01G23SHBQB
	opts := []slack.MsgOption{slack.MsgOptionText("<@U01G23SHBQB> 予報", false)}
	if payload.Event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(payload.Event.ThreadTimeStamp))
	}
	_, _, err := bot.SlackAPI.PostMessage(payload.Event.Channel, opts...)
	log.Printf("[amesh] %v", err)

}

var (
	tplEquipsManagementSummary = template.Must(template.New("").Parse(`備品管理状況は以下の通り:
{{if len .Unmanaged}}*【1度も回答がついていない備品】*
{{range .Unmanaged}}- {{.Name}}
{{end}}--------------{{end}}
{{if len .NotUpdated}}*【直近7日間で回答がついていない練習用備品】*
{{range .NotUpdated}}- {{.Name}}
{{end}}--------------{{end}}
https://hub.triax.football/equips`))

	tplReadCheck = template.Must(template.New("").Parse(`このメッセージに返信が期待されている人:
{{range .Expected}}{{.}} {{end}}
しかしリアクションしてない人
{{range .NotReacted}}{{.}} {{end}}`))
)
