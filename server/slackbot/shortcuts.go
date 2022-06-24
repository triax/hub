package slackbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/slack-go/slack"
)

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
	switch payload.CallbackID {
	case "translate_to_eng":
		err = bot.TranslateToEng(req.Context(), payload)
	default:
	}

	log.Println(err)
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
