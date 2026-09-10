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

// SlashCommands は Slack のスラッシュコマンドの入口。command で分岐する。
func (bot Bot) SlashCommands(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	defer req.Body.Close()

	// Events 側（Webhook）は VerificationToken を見ているのに、ここだけ素通しだった。
	// URL を知っていれば誰でも bot に DM とチャンネル投稿をさせられる状態だったので塞ぐ（#691）。
	// 未設定の環境（ローカル開発）では検証しない。本番では必ず設定されている。
	//
	// stellar:debt(scope) VerificationToken は Slack 側で deprecated 扱い。
	// upgrade: signing secret（slack.NewSecretsVerifier）へ移行する。新しい secret の
	// プロビジョニングが要るので別 Issue に切る
	if bot.VerificationToken != "" && req.Form.Get("token") != bot.VerificationToken {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch req.Form.Get("command") {
	case "/premortem":
		bot.onSlashPremortem(w, req)
	default:
		// コマンド名を判定に使わず既定へ落とす。既存の「ありがとう」を壊さないため。
		bot.onSlashThankYou(w, req)
	}
}

// onSlashThankYou は「ありがとう」コマンド。本文中のメンション宛に匿名 DM を送る。
func (bot Bot) onSlashThankYou(w http.ResponseWriter, req *http.Request) {
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
