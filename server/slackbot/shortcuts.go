package slackbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
)

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

	var err error
	switch {
	case payload.CallbackID == "translate_to_eng":
		err = bot.TranslateToEng(req.Context(), payload)
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
		client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
		if err != nil {
			fmt.Println(err)
			return
		}
		defer client.Close()
		custody := &models.Custody{
			MemberID:  mid,
			Timestamp: time.Now().Unix() * 1000,
		}
		id, _ := strconv.ParseInt(eid, 10, 32)
		_, err = client.Put(ctx, datastore.IncompleteKey(models.KindCustody, datastore.IDKey(models.KindEquip, id, nil)), custody)
		if err != nil {
			fmt.Println(err)
			return
		}

		http.Post(payload.ResponseURL, "application/json", strings.NewReader(fmt.Sprintf(`{"text":":check: %s :bow:"}`, eid)))
	}

	// for i, act := range payload.ActionCallback.BlockActions {
	// 	fmt.Printf("[%d]\t%+v\n", i, act)
	// 	fmt.Println(act.SelectedUser)
	// }

	if err != nil {
		log.Println(err)
	}
}

func (bot Bot) TranslateToEng(ctx context.Context, payload slack.InteractionCallback) error {
	// {{{ TODO: module
	params := url.Values{}
	params.Add("auth_key", os.Getenv("DEEPL_API_TOKEN"))
	params.Add("text", payload.Message.Text)
	params.Add("target_lang", "EN")
	params.Add("source_lang", "JA")
	deepl, err := http.NewRequestWithContext(ctx, "GET", "https://api-free.deepl.com/v2/translate"+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	fmt.Println(deepl.URL.String(), err)
	res, err := http.DefaultClient.Do(deepl)
	if err != nil {
		return err
	}
	translated := struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}{}
	json.NewDecoder(res.Body).Decode(&translated)
	if len(translated.Translations) == 0 {
		return fmt.Errorf("failed to translate with 0 entry")
	}
	// }}}

	text := translated.Translations[0].Text

	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if payload.MessageTs != "" {
		opts = append(opts, slack.MsgOptionTS(payload.MessageTs))
	}
	a, b, err := bot.SlackAPI.PostMessage(payload.Channel.ID, opts...)
	fmt.Println(a, b, err)
	return nil
}
