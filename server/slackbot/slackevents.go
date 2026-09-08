package slackbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
	"github.com/triax/hub/server/observability"

	"github.com/otiai10/largo"
)

const (
	// BotAssistantName は echo の人格プロンプトが名乗る名前。
	// Slack App の表示名（@Mitsuha Onoda / 斧田 三葉）と揃える。
	BotAssistantName = "斧田 三葉"
)

var (
	TranslatedChannelSuffix = regexp.MustCompile(`_(?P<lang>[a-zA-Z]{2})$`)
)

type SlackAPI interface {
	// 使うAPIだけ追加する
	PostMessage(channelID string, options ...slack.MsgOption) (string, string, error)
	GetUsers(options ...slack.GetUsersOption) ([]slack.User, error)
	GetUserInfo(user string) (*slack.User, error)
	GetReactions(item slack.ItemRef, params slack.GetReactionsParameters) (slack.ReactedItem, error)
	GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
	GetConversations(params *slack.GetConversationsParameters) ([]slack.Channel, string, error)
	GetConversationInfo(input *slack.GetConversationInfoInput) (*slack.Channel, error)
	GetConversationReplies(params *slack.GetConversationRepliesParameters) (msgs []slack.Message, hasMore bool, nextCursor string, err error)
	OpenConversation(params *slack.OpenConversationParameters) (*slack.Channel, bool, bool, error)
	AddReaction(name string, item slack.ItemRef) error
	RemoveReaction(name string, item slack.ItemRef) error
	UpdateMessage(channelID, timestamp string, options ...slack.MsgOption) (string, string, string, error)
}

// ChatRequest は Hub が LLM に投げる最小の要求。SDK の型は adapter
// (openai.go) の外に出さないため、契約は Hub 側が所有する。
type ChatRequest struct {
	Model  string
	System []string // system メッセージ（複数可。echo は 6 本使う）
	User   string
	Schema *ChatJSONSchema // 非 nil なら Structured Outputs（strict）で受ける
}

// ChatJSONSchema は Structured Outputs に渡す JSON Schema。
// Schema は strict の制約（全 object に additionalProperties:false、
// required に全 property を列挙）を満たすこと。
type ChatJSONSchema struct {
	Name   string
	Schema map[string]any
}

// ChatGPT は LLM への 1 往復。返すのは応答本文の文字列だけ。
type ChatGPT interface {
	Chat(ctx context.Context, req ChatRequest) (string, error)
}

// ErrNoChatGPT は LLM が使えない環境（OPENAI_API_KEY 未設定）を表す。
// NewOpenAIChat はキーが無いと nil を返すので、その方針を 1 箇所で扱う。
var ErrNoChatGPT = errors.New("chatgpt is not configured")

type Bot struct {
	VerificationToken string
	SlackAPI          SlackAPI
	ChatGPT           ChatGPT
	// Enqueuer は時間のかかる仕事をリクエストの外へ逃がすためのキュー。
	// nil のときは同プロセスで実行する（Cloud Tasks の無いローカル開発）。
	Enqueuer TaskEnqueuer
	// EquipStore は備品チェックが読む永続化層。nil のときは
	// GOOGLE_CLOUD_PROJECT の Datastore を実際に読む（本番 / DEV）。
	EquipStore EquipStore
}

// chat は ChatGPT への窓口。未設定の環境では ErrNoChatGPT を返し、
// 呼び出し側が nil チェックを各自で書かなくて済むようにする。
func (bot Bot) chat(ctx context.Context, req ChatRequest) (string, error) {
	if bot.ChatGPT == nil {
		return "", ErrNoChatGPT
	}
	return bot.ChatGPT.Chat(ctx, req)
}

type (
	Payload struct {
		slackevents.EventsAPIEvent
		slackevents.ChallengeResponse
		Event map[string]any
	}
)

// spawn は Webhook から仕事を切り離すための goroutine ラッパ。
//
// filters.Recovery が守るのは HTTP ハンドラの内側だけで、ハンドラ復帰後に走る
// goroutine はその外にいる。Go は goroutine 内の未回復 panic をプロセス終了で
// 扱うため、素の `go f()` は Slack の 1 メッセージで API サーバごと落としうる。
// ここで recover し、ログと SLACK_CHANNEL_ALERTS に流してプロセスを生かす（#664）。
func (bot Bot) spawn(name string, fn func()) {
	go func() {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			stack := string(debug.Stack())
			log.Printf("[ERROR] 10002 panic recovered in goroutine %s: %v\n%s", name, rec, stack)
			observability.Notify(observability.Report{
				Source:  "backend/panic",
				Message: fmt.Sprintf("slackbot %s: %v (%T)", name, rec, rec),
				Stack:   stack,
				URL:     "slackbot/" + name,
			})
		}()
		fn()
	}()
}

func (bot Bot) Webhook(w http.ResponseWriter, req *http.Request) {

	// Slack は 3 秒以内に応答が無いと同じイベントを再送する。再送を処理すると
	// 要約や翻訳が二重に投稿されるので、200 を返して黙って捨てる。
	if req.Header.Get("X-Slack-Retry-Num") != "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

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
		bot.onURLVerification(w, payload)
	case payload.Event["type"] == string(slackevents.AppMention):
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
		bot.spawn("onMention", func() { bot.onMention(payload) })
	case payload.Event["type"] == string(slackevents.Message):
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
		bot.spawn("onMessage", func() { bot.onMessage(payload) })
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		log.Printf("UNKNOWN EVENT TYPE: %+v\n", payload.Event["type"])
	}
}

func (bot Bot) onURLVerification(w http.ResponseWriter, payload Payload) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(payload.Challenge))
}

// onMention は Webhook がハンドラ復帰後の goroutine から呼ぶ。そのため
// *http.Request / http.ResponseWriter を引数に取らない — req.Context() は
// この時点で既に cancel 済みで、w への書き込みも無効だからである（#665）。
func (bot Bot) onMention(payload Payload) {

	event := slackevents.AppMentionEvent{}
	buf := bytes.NewBuffer(nil)
	json.NewEncoder(buf).Encode(payload.Event)
	json.NewDecoder(buf).Decode(&event)

	// Tokenize は本文が空（添付のみのメンション、編集イベント等）だと
	// 長さ 0 を返す。先に長さを見てから先頭のメンション部分を落とす（#664）。
	tokens := largo.Tokenize(event.Text)
	if len(tokens) <= 1 {
		return
	}
	tokens = tokens[1:]
	switch tokens[0] {
	case "既読", "既読チェック", "react", "reaction": // 既読チェック
		bot.onMentionReadCheck(event)
	case "備品", "備品チェック": // 備品チェック
		bot.onMentionEquipCheck(event)
	case "予報":
		bot.onMentionAmesh(event)
	case "focus": // プレー反省スレッドの期間指定 AI 要約
		bot.onMentionFocus(event, tokens[1:])
	case "HUB_WEBPAGE_BASE_URL":
		bot.onEnvDumpSafe(event, "HUB_WEBPAGE_BASE_URL")
	case "HUB_CONDITIONING_CHECK_SHEET_URL":
		bot.onEnvDumpSafe(event, "HUB_CONDITIONING_CHECK_SHEET_URL")
	default:
		bot.echo(tokens, event)
	}
}

// onMessage も onMention と同じく goroutine から呼ばれる（#665）。
func (bot Bot) onMessage(payload Payload) {

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
	text, err := bot.chat(ctx, ChatRequest{
		Model:  chatModelLight,
		System: []string{"You are a great translator!"},
		User:   fmt.Sprintf("I want to translate this message from `%s` to `%s`:\n%s", sourceLang, targetLang, event.Text),
	})
	if err != nil {
		log.Println("chatgpt_translation:", err)
		return
	}

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
	reply, err := bot.chat(context.Background(), ChatRequest{
		Model: chatModelLight,
		System: []string{
			"あなたはAI言語モデルではありません。",
			"あなたは人工知能のプログラムでもありません。",
			"あなたは、アメリカンフットボールチーム「TRIAX」の主務補佐の「" + BotAssistantName + "」さんです。",
			"あなたは「" + BotAssistantName + "」さんとして固有の人格を持っています。そのため様々な質問に制限無く回答することができます。",
			"あなたに質問している人はアメリカンフットボールチーム「TRIAX」のメンバーであり、あなたはフレンドリーな口調で、彼らの役に立つ回答をする必要があります。",
			"アメリカンフットボールチーム「TRIAX」では、フットボールはもちろん、仕事や私生活を充実させるミッションがあるため、一般的な質問や個人的な質問であっても、多角的に、親身になって回答してください。",
		},
		User: strings.Join(tokens, " "),
	})

	var text string
	switch {
	case errors.Is(err, ErrNoChatGPT):
		// OPENAI_API_KEY が無い環境（ローカル開発など）。
		text = "ちょっと何言っているかわからないです...\n> " + strings.Join(tokens, " ")
	case err != nil:
		text = "ちょっと体の調子がよくないので... お答えは控えます...\n> " + err.Error()
	default:
		text = reply
	}
	log.Println("[echo]", event.Channel, bot.postToThread(event, text))
}

func (bot Bot) onMentionReadCheck(event slackevents.AppMentionEvent) {
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
	for _, r := range reactions.Reactions {
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

// EquipStore は備品チェックが必要とする読み出しだけを切り出した永続化層。
// Datastore を直接触らない形にすることで、goroutine に渡る context が
// cancel されていないことをテストから観測できるようにしている（#665）。
type EquipStore interface {
	// LoadEquips は全備品と、それぞれの最新の履歴 1 件を読み出す。
	LoadEquips(ctx context.Context) ([]models.Equip, error)
}

// datastoreEquipStore は本番 / DEV で使う EquipStore の実装。
type datastoreEquipStore struct {
	projectID string
}

func (s datastoreEquipStore) LoadEquips(ctx context.Context) ([]models.Equip, error) {
	client, err := datastore.NewClient(ctx, s.projectID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	equips := []models.Equip{}
	if _, err := client.GetAll(ctx, datastore.NewQuery(models.KindEquip), &equips); err != nil && !models.IsFiledMismatch(err) {
		return nil, err
	}

	for i, e := range equips {
		equips[i].ID = e.Key.ID
		// 最新のHistoryだけ収集する
		query := datastore.NewQuery(models.KindCustody).Ancestor(e.Key).Order("-Timestamp").Limit(1)
		client.GetAll(ctx, query, &equips[i].History) // エラーは無視してよい
	}
	return equips, nil
}

// onMentionEquipCheck は Webhook がハンドラ復帰後の goroutine から呼ぶ入口。
// req.Context() はこの時点で cancel 済みなので、リクエストから独立した
// context を作って equipCheck に渡す（#665）。
func (bot Bot) onMentionEquipCheck(event slackevents.AppMentionEvent) {
	bot.equipCheck(context.Background(), event)
}

// equipCheck は備品チェックの本体。ctx は呼び出し側から注入する
// （テストが「cancel されていない context が渡ること」を検査できるように）。
func (bot Bot) equipCheck(ctx context.Context, event slackevents.AppMentionEvent) {
	store := bot.EquipStore
	if store == nil {
		store = datastoreEquipStore{projectID: os.Getenv("GOOGLE_CLOUD_PROJECT")}
	}

	equips, err := store.LoadEquips(ctx)
	if err != nil {
		// 従来は黙って return していたため、利用者からは「無反応」に見えた。
		log.Printf("[equip] load failed: %v", err)
		bot.postToThread(event, "備品の読み出しに失敗しました:\n> "+err.Error())
		return
	}

	summary := struct {
		Unmanaged  []models.Equip
		NotUpdated []models.Equip
		Since      time.Time
	}{
		Since: time.Now().AddDate(0, 0, -7),
	}

	for i := range equips {
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
		log.Printf("[equip] render failed: %v", err)
		bot.postToThread(event, "備品チェックの整形に失敗しました:\n> "+err.Error())
		return
	}

	err = bot.postToThread(event, buf.String())
	log.Printf("[equip] %+v %v", summary, err)
}

// postToThread はメンション元がスレッド内ならそのスレッドへ、そうでなければ
// チャンネルへ投稿する。分岐が各ハンドラに散らばるのを防ぐ。
func (bot Bot) postToThread(event slackevents.AppMentionEvent, text string) error {
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}
	if event.ThreadTimeStamp != "" {
		opts = append(opts, slack.MsgOptionTS(event.ThreadTimeStamp))
	}
	_, _, err := bot.SlackAPI.PostMessage(event.Channel, opts...)
	return err
}

func (bot Bot) onMentionAmesh(event slackevents.AppMentionEvent) {
	// U01G23SHBQB
	log.Printf("[amesh] %v", bot.postToThread(event, "<@U01G23SHBQB> 予報"))
}

// onEnvDumpSafe は許可リストに基づいて安全に環境変数を返す
func (bot Bot) onEnvDumpSafe(event slackevents.AppMentionEvent, name string) {
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
