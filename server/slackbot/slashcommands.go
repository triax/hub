package slackbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
)

var (
	mentionExp = regexp.MustCompile(`@(?P<name>[\S]+)`)
)

func (bot Bot) SlashCommands(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	defer req.Body.Close()
	text := req.Form.Get("text")
	names := []any{}
	idx := mentionExp.SubexpIndex("name")
	for _, m := range mentionExp.FindAllStringSubmatch(text, -1) {
		names = append(names, m[idx])
	}
	if len(names) == 0 {
		http.Post(req.Form.Get("response_url"), "application/json", strings.NewReader(`{
			"text": "誰に対するありがとうか、メンションで指定してください。本人には匿名のDMで通知されます。"
		}`))
		return
	}

	ctx := context.Background()
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		b, err := json.Marshal(map[string]string{
			"text": fmt.Sprintf("データストアに接続できませんでした。 @ten までご連絡ください。\n```%s```", err.Error()),
		})
		if err != nil {
			fmt.Println("[ERROR]", 6003, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Post(req.Form.Get("response_url"), "application/json", bytes.NewReader(b))
		return
	}
	defer client.Close()

	members := []models.Member{}
	query := datastore.NewQuery(models.KindMember).FilterField("Slack.Name", "in", names)
	if _, err := client.GetAll(ctx, query, &members); err != nil {
		b, err := json.Marshal(map[string]string{
			"text": fmt.Sprintf("データストアからのデータ取得に失敗しました。 @ten までご連絡ください。\n```%s```", err.Error()),
		})
		if err != nil {
			fmt.Println("[ERROR]", 6003, err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Post(req.Form.Get("response_url"), "application/json", bytes.NewReader(b))
		return
	}
	w.WriteHeader(http.StatusOK) // とりあえずここまででSlackにレスポンスを返す

	senderID := req.Form.Get("user_id")
	message := mentionExp.ReplaceAllString(text, "")
	announce := ""
	feedback := "ありがとう！を "
	for _, m := range members {
		// 2-a) conversations.open to get DM channel
		ch, _, _, err := bot.SlackAPI.OpenConversation(&slack.OpenConversationParameters{
			Users: []string{m.Slack.ID},
		})
		if err != nil {
			fmt.Println("[ERROR]", 6004, err.Error())
			continue
		}
		// 2-b) sendMessage to the DM
		_, _, err = bot.SlackAPI.PostMessage(
			ch.ID,
			slack.MsgOptionText(fmt.Sprintf("<@%s>さんからありがとう！が届きました。\n> %s", senderID, message), false),
		)
		if err != nil {
			fmt.Println("[ERROR]", 6005, err.Error())
			continue
		}

		feedback += fmt.Sprintf("<@%s> さん ", m.Slack.ID)
		announce += ":heart:"
	}
	feedback += "に伝えました。\n" + message

	http.Post(req.Form.Get("response_url"), "application/json", strings.NewReader(`{"text":"`+feedback+`"}`))
	bot.SlackAPI.PostMessage("thankyou", slack.MsgOptionText(announce, false))
}
