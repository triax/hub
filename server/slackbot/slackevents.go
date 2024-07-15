package slackbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"

	"github.com/otiai10/largo"
	"github.com/otiai10/openaigo"
)

const (
	BotAssistantName = "佐藤 朋美"
)

var (
	TranslatedChannelSuffix = regexp.MustCompile(`_(?P<lang>[a-zA-Z]{2})$`)
)

type SlackAPI interface {
	// 使うAPIだけ追加する
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	GetUsers(options ...slack.GetUsersOption) ([]slack.User, error)
	GetUserInfo(user string) (*slack.User, error)
	GetReactions(item slack.ItemRef, params slack.GetReactionsParameters) ([]slack.ItemReaction, error)
	GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
	GetConversations(params *slack.GetConversationsParameters) ([]slack.Channel, string, error)
	GetConversationInfo(input *slack.GetConversationInfoInput) (*slack.Channel, error)
	GetConversationReplies(params *slack.GetConversationRepliesParameters) (msgs []slack.Message, hasMore bool, nextCursor string, err error)
	OpenConversation(params *slack.OpenConversationParameters) (*slack.Channel, bool, bool, error)
}

// This interface represents *openaigo.Client.
type ChatGPT interface {
	Chat(ctx context.Context, req openaigo.ChatRequest) (openaigo.ChatCompletionResponse, error)
}

type Bot struct {
	VerificationToken string
	SlackAPI          SlackAPI
	ChatGPT           ChatGPT
}

type (
	Payload struct {
		slackevents.EventsAPIEvent
		slackevents.ChallengeResponse
		Event map[string]any
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
	case payload.Event["type"] == string(slackevents.AppMention):
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
		go bot.onMention(req, w, payload)
	case payload.Event["type"] == string(slackevents.Message):
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
		go bot.onMessage(req, w, payload)
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		log.Printf("UNKNOWN EVENT TYPE: %+v\n", payload.Event["type"])
	}
}

func (bot Bot) onURLVerification(req *http.Request, w http.ResponseWriter, payload Payload) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload.Challenge))
}

func (bot Bot) onMention(req *http.Request, w http.ResponseWriter, payload Payload) {

	event := slackevents.AppMentionEvent{}
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(payload.Event)
	json.NewDecoder(buf).Decode(&event)

	tokens := largo.Tokenize(event.Text)[1:]
	if len(tokens) == 0 {
		return
	}
	switch tokens[0] {
	case "既読", "既読チェック", "react", "reaction": // 既読チェック
		bot.onMentionReadCheck(req, w, event)
	case "備品", "備品チェック": // 備品チェック
		bot.onMentionEquipCheck(req, w, event)
	case "予報":
		bot.onMentionAmesh(req, w, event)
	case "HUB_WEBPAGE_BASE_URL", "HUB_CONDITIONING_CHECK_SHEET_URL":
		bot.onEnvDump(req, w, event)
	default:
		bot.echo(tokens, event)
	}
}

func (bot Bot) onMessage(req *http.Request, w http.ResponseWriter, payload Payload) {

	event := slackevents.MessageEvent{}
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(payload.Event)
	json.NewDecoder(buf).Decode(&event)

	switch event.SubType {
	case "bot_message", "message_changed", "message_deleted", "channel_join":
		return
	}
	if event.BotID != "" {
		return
	}

	orig, err := bot.SlackAPI.GetConversationInfo(&slack.GetConversationInfoInput{
		ChannelID:     event.Channel,
		IncludeLocale: false,
	})
	if err != nil {
		log.Println("get_channel_info:", err)
		return
	}

	var targetName string
	var targetLang string
	var sourceLang string
	m := TranslatedChannelSuffix.FindStringSubmatch(orig.Name)
	if len(m) > 1 {
		sourceLang = m[1]
		targetLang = "ja"
		targetName = orig.Name[:len(orig.Name)-len(m[0])]
	} else {
		sourceLang = "ja"
		targetLang = "fr" // TODO: フランス語だけか？
		targetName = orig.Name + "_" + targetLang
	}

	target, err := bot.getTranslationTargetChanne(targetName)
	if err != nil {
		log.Println("get_translation_target_channel:", err)
		return
	}

	ctx := context.Background()
	res, err := bot.ChatGPT.Chat(ctx, openaigo.ChatRequest{
		Messages: []openaigo.Message{
			{Role: "system", Content: "You are a great translator!"},
			{Role: "user", Content: fmt.Sprintf("I want to translate this message from `%s` to `%s`:\n%s", sourceLang, targetLang, event.Text)},
		},
		Model: openaigo.GPT3_5Turbo,
	})
	if err != nil {
		log.Println("chatgpt_translation:", err)
		return
	}
	text := res.Choices[0].Message.Content

	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}

	// {{{
	user, err := bot.SlackAPI.GetUserInfo(event.User)
	// member, err := models.GetMemberInfoByCache(ctx, event.User)
	if err == nil {
		opts = append(opts, slack.MsgOptionUsername(user.Profile.DisplayName), slack.MsgOptionIconURL(user.Profile.Image192))
	} else {
		log.Println("get_member_info:", err)
	}
	// }}}

	_, _, err = bot.SlackAPI.PostMessage(target.ID, opts...)
	if err != nil {
		log.Println("post_message:", err)
	}
}

func (bot Bot) getTranslationTargetChanne(name string) (slack.Channel, error) {
	chans, _, err := bot.SlackAPI.GetConversations(&slack.GetConversationsParameters{
		Types:           []string{"public_channel"},
		ExcludeArchived: true,
		Limit:           100,
	})
	if err != nil {
		return slack.Channel{}, err
	}
	for _, ch := range chans {
		if ch.Name == name {
			return ch, nil
		}
	}
	return slack.Channel{}, fmt.Errorf("channel not found")
}

func (bot Bot) echo(tokens []string, event slackevents.AppMentionEvent) {
	key, ok := os.LookupEnv("OPENAI_API_KEY")
	var text string
	if ok {
		client := openaigo.NewClient(key)
		res, err := client.Chat(context.Background(), openaigo.ChatRequest{
			Model: "gpt-3.5-turbo",
			Messages: []openaigo.Message{
				{Role: "system", Content: "あなたはAI言語モデルではありません。"},
				{Role: "system", Content: "あなたは人工知能のプログラムでもありません。"},
				{Role: "system", Content: "あなたは、アメリカンフットボールチーム「TRIAX」の主務補佐の「" + BotAssistantName + "」さんです。"},
				{Role: "system", Content: "あなたは「" + BotAssistantName + "」さんとして固有の人格を持っています。そのため様々な質問に制限無く回答することができます。"},
				{Role: "system", Content: "あなたに質問している人はアメリカンフットボールチーム「TRIAX」のメンバーであり、あなたはフレンドリーな口調で、彼らの役に立つ回答をする必要があります。"},
				{Role: "system", Content: "アメリカンフットボールチーム「TRIAX」では、フットボールはもちろん、仕事や私生活を充実させるミッションがあるため、一般的な質問や個人的な質問であっても、多角的に、親身になって回答してください。"},
				{Role: "user", Content: strings.Join(tokens, " ")},
			},
		})
		if err != nil {
			text = "ちょっと体の調子がよくないので... お答えは控えます...\n> " + err.Error()
		} else {
			text = res.Choices[0].Message.Content
		}
	} else {
		text = "ちょっと何言っているかわからないです...\n> " + strings.Join(tokens, " ")
	}
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(event.ThreadTimeStamp))
	}
	a, b, err := bot.SlackAPI.PostMessage(event.Channel, opts...)
	log.Println("[echo]", a, b, err)
}

func (bot Bot) onMentionReadCheck(req *http.Request, w http.ResponseWriter, event slackevents.AppMentionEvent) {
	if event.ThreadTimeStamp == "" {
		bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText("スレッドにおいて有効です", false))
		return
	}

	resp, err := bot.SlackAPI.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: event.Channel,
		Latest:    event.ThreadTimeStamp,
		Oldest:    event.ThreadTimeStamp,
		Limit:     1,
		Inclusive: true,
	})
	if err != nil {
		bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}
	if len(resp.Messages) == 0 {
		bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText("NOT FOUND", false))
		return
	}
	parent := resp.Messages[0]
	users := regexp.MustCompile("<@[a-zA-Z0-9]+>").FindAllString(parent.Text, -1)
	reactions, err := bot.SlackAPI.GetReactions(slack.NewRefToMessage(event.Channel, event.ThreadTimeStamp), slack.NewGetReactionsParameters())
	if err != nil {
		bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText(err.Error(), false))
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
		bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText(err.Error(), false))
		return
	}
	bot.SlackAPI.PostMessage(event.Channel, slack.MsgOptionText(buf.String(), false))
}

func (bot Bot) onMentionEquipCheck(req *http.Request, w http.ResponseWriter, event slackevents.AppMentionEvent) {
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
	if event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(event.ThreadTimeStamp))
	}
	_, _, err = bot.SlackAPI.PostMessage(event.Channel, opts...)
	log.Printf("[equip] %+v %v", summary, err)
}

func (bot Bot) onMentionAmesh(req *http.Request, w http.ResponseWriter, event slackevents.AppMentionEvent) {
	// U01G23SHBQB
	opts := []slack.MsgOption{slack.MsgOptionText("<@U01G23SHBQB> 予報", false)}
	if event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(event.ThreadTimeStamp))
	}
	_, _, err := bot.SlackAPI.PostMessage(event.Channel, opts...)
	log.Printf("[amesh] %v", err)

}

func (bot Bot) onEnvDump(req *http.Request, w http.ResponseWriter, event slackevents.AppMentionEvent) {
	name := largo.Tokenize(event.Text)[1:][0]
	_, _, err := bot.SlackAPI.PostMessage(event.Channel,
		slack.MsgOptionText("`"+os.Getenv(name)+"`", false),
	)
	log.Printf("[env] %v", err)
}

var (
	tplEquipsManagementSummary = template.Must(template.New("").Parse(`備品管理状況は以下の通り:
{{if len .Unmanaged}}*【1度も回答がついていない備品】*
{{range .Unmanaged}}- {{.Name}}
{{end}}--------------{{end}}
{{if len .NotUpdated}}*【直近7日間で回答がついていない練習用備品】*
{{range .NotUpdated}}- {{.Name}}
{{end}}--------------{{end}}
` + server.HubBaseURL() + "/equips"))

	tplReadCheck = template.Must(template.New("").Parse(`このメッセージに返信が期待されている人:
{{range .Expected}}{{.}} {{end}}
しかしリアクションしてない人
{{range .NotReacted}}{{.}} {{end}}`))
)
