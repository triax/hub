package slackbot

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
)

var (
	mentionExpEscaped = regexp.MustCompile(`<@(?P<name>[\S]+)>`)
)

func (bot Bot) SlashCommands(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	defer req.Body.Close()
	text := req.Form.Get("text")
	ids := []string{}
	idx := mentionExpEscaped.SubexpIndex("name")
	for _, m := range mentionExpEscaped.FindAllStringSubmatch(text, -1) {
		ids = append(ids, m[idx])
	}
	if len(ids) == 0 {
		postSlackJSON(req.Form.Get("response_url"), "誰に対するありがとうか、メンションで指定してください。本人には匿名のDMで通知されます。")
		return
	}

	w.WriteHeader(http.StatusOK) // とりあえずここまででSlackにレスポンスを返す

	senderID := req.Form.Get("user_id")
	message := mentionExpEscaped.ReplaceAllString(text, "")
	announce := ""
	feedback := "ありがとう！を "
	for _, id := range ids {
		ch, _, _, err := bot.SlackAPI.OpenConversation(&slack.OpenConversationParameters{
			Users: []string{strings.Split(id, "|")[0]},
		})
		if err != nil {
			log.Printf("open conversation error: %v", err)
			continue
		}
		_, _, err = bot.SlackAPI.PostMessage(
			ch.ID,
			slack.MsgOptionText(fmt.Sprintf("<@%s>さんからありがとう！が届きました。\n> %s", senderID, message), false),
		)
		if err != nil {
			log.Printf("post message error: %v", err)
			continue
		}

		feedback += fmt.Sprintf("<@%s> さん ", id)
		announce += ":heart:"
	}
	feedback += "に伝えました。\n" + message

	postSlackJSON(req.Form.Get("response_url"), feedback)
	bot.SlackAPI.PostMessage("thankyou", slack.MsgOptionText(announce, false))
}
