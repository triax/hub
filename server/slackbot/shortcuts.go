package slackbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/otiai10/openaigo"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
)

// postSlackJSON はSlackのresponse_urlに安全にJSONを送信する
func postSlackJSON(responseURL, text string) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		log.Printf("failed to marshal slack response: %v", err)
		return
	}
	http.Post(responseURL, "application/json", bytes.NewReader(body))
}

// TODO: 名前は正しくない
func (bot Bot) Shortcuts(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()
	payload := slack.InteractionCallback{}
	if err := json.Unmarshal([]byte(req.Form.Get("payload")), &payload); err != nil {
		w.WriteHeader(200)
		return
	}

	if payload.Token != os.Getenv("SLACK_BOT_EVENTS_VERIFICATION_TOKEN") {
		w.WriteHeader(200)
		return
	}

	w.WriteHeader(200)

	ctx := context.Background()

	var err error
	switch { // TODO: 言語コードは2文字に統一したい
	case payload.CallbackID == "translate_to_eng" || payload.CallbackID == "translate_to_en":
		err = bot.Translate(ctx, payload, "EN")
	case payload.CallbackID == "translate_to_jpn" || payload.CallbackID == "translate_to_ja":
		err = bot.Translate(ctx, payload, "JA")
	case payload.CallbackID == "translate_to_fra" || payload.CallbackID == "translate_to_fr":
		err = bot.Translate(ctx, payload, "FR")
	case payload.Type == "block_actions":
		if len(payload.ActionCallback.BlockActions) == 0 {
			return
		}
		action := payload.ActionCallback.BlockActions[0]
		u, err := url.Parse(action.ActionID)
		if err != nil {
			fmt.Println(err)
			return
		}
		eid := u.Query().Get("eid")
		mid := action.SelectedUser
		// ev := u.Query().Get("ev")

		ctx := context.Background()
		member, err := models.GetMemberInfoByCache(ctx, mid)
		if err != nil {
			fmt.Println(err) // TODO: Error log
			return
		}

		client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
		if err != nil {
			fmt.Println(err) // TODO: Error log
			return
		}
		defer client.Close()
		custody := &models.Custody{
			MemberID:  mid,
			Timestamp: time.Now().Unix() * 1000,
		}
		eidnumeric, _ := strconv.Atoi(eid)
		if _, err = client.Put(ctx, datastore.IncompleteKey(
			models.KindCustody,
			datastore.IDKey(models.KindEquip, int64(eidnumeric), nil)),
			custody,
		); err != nil {
			fmt.Println(err) // TODO: Error log
			return
		}

		postSlackJSON(payload.ResponseURL, fmt.Sprintf(":white_check_mark: %s ⇒ %s", payload.Message.Text, member.Name()))

		// TODO: 全回答フィードバック
	}

	if err != nil {
		log.Println(err)
	}
}

// Translate method translate original message to given language by OpenAI API,
// and post it in a thread of the original message.
func (bot Bot) Translate(ctx context.Context, payload slack.InteractionCallback, lang string) error {
	res, err := bot.ChatGPT.Chat(ctx, openaigo.ChatRequest{
		Messages: []openaigo.Message{
			{Role: "system", Content: "You are a great translator!"},
			{Role: "user", Content: fmt.Sprintf("I want to translate this message to `%s`:\n%s", lang, payload.Message.Text)},
		},
		Model: openaigo.GPT3_5Turbo,
	})
	if err != nil {
		return fmt.Errorf("chatgpt_translation: %v", err)
	}
	text := res.Choices[0].Message.Content
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("json_marshal: %v", err)
	}
	slackres, err := http.Post(payload.ResponseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	return slackres.Body.Close()
}
