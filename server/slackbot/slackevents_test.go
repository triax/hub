package slackbot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/triax/hub/server/models"
)

func mentionPayload(text, channel, ts, threadTS string) Payload {
	return Payload{Event: map[string]any{
		"type":      "app_mention",
		"text":      text,
		"channel":   channel,
		"ts":        ts,
		"thread_ts": threadTS,
	}}
}

// AC-5: `focus` 以外のトークンで focus が起動しないこと（既存の分岐が変わらないこと）。
//
// `備品` は Datastore へ接続するため、ここでは対象外にしている
// （本番プロジェクトへ実アクセスしうるので自動テストでは踏まない）。
func TestOnMention_Dispatch(t *testing.T) {
	cases := []struct {
		name        string
		text        string
		wantEnqueue int
	}{
		{"focus", "<@BOT> focus 8/8", 1},
		{"既読", "<@BOT> 既読", 0},
		{"予報", "<@BOT> 予報", 0},
		{"echo（既定）", "<@BOT> こんにちは", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			api := newFakeSlackAPI()
			enq := newFakeEnqueuer()
			bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}

			bot.onMention(mentionPayload(c.text, "C1", testMentionTS, ""))

			if got := enq.count(); got != c.wantEnqueue {
				t.Fatalf("enqueue = %d, want %d", got, c.wantEnqueue)
			}
			if c.wantEnqueue == 0 && len(api.added) != 0 {
				t.Fatalf("focus 以外で 👀 が付いている: %v", api.added)
			}
		})
	}
}

// AC-5 / AC-3: focus は 👀 を付けて、期間つきの仕事をキューへ積むだけで返る。
func TestOnMentionFocus_Enqueues(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}

	bot.onMentionFocus(mentionEvent("<@BOT> focus 8/8", "C1", testMentionTS, ""), []string{"8/8"})

	if enq.count() != 1 {
		t.Fatalf("enqueue = %d, want 1", enq.count())
	}
	if enq.uris[0] != FocusTaskURI {
		t.Fatalf("relativeURI = %q, want %q", enq.uris[0], FocusTaskURI)
	}
	if !strings.Contains(enq.bodies[0], `"channel":"C1"`) || !strings.Contains(enq.bodies[0], `"mention_ts":"`+testMentionTS+`"`) {
		t.Fatalf("payload が期待どおりでない: %s", enq.bodies[0])
	}
	if joined(api.added) != "eyes" {
		t.Fatalf("👀 が付いていない: %v", api.added)
	}
	if len(api.posted) != 0 {
		t.Fatalf("受付段階で投稿している: %+v", api.posted)
	}
}

// enqueue に失敗したらスレッドにそのまま返し、👀 を外す。
func TestOnMentionFocus_EnqueueFailure(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	enq.err = errStub("CLOUD_TASKS_LOCATION is not set")
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}

	bot.onMentionFocus(mentionEvent("<@BOT> focus", "C1", testMentionTS, ""), nil)

	if len(api.posted) != 1 || !strings.Contains(api.posted[0].Text(), "CLOUD_TASKS_LOCATION") {
		t.Fatalf("enqueue 失敗がスレッドに返っていない: %+v", api.posted)
	}
	if joined(api.removed) != "eyes" {
		t.Fatalf("👀 が外れていない: %v", api.removed)
	}
}

// AC-6: Slack の再送（X-Slack-Retry-Num）は 200 を返して何もしない。
func TestWebhook_IgnoresSlackRetry(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	bot := Bot{VerificationToken: "vt", SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}

	body := `{"token":"vt","event":{"type":"app_mention","text":"<@BOT> focus 8/8","channel":"C1","ts":"` + testMentionTS + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	req.Header.Set("X-Slack-Retry-Num", "1")
	rec := httptest.NewRecorder()

	bot.Webhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if enq.count() != 0 {
		t.Fatal("再送なのに enqueue している")
	}
	if len(api.added) != 0 || len(api.posted) != 0 {
		t.Fatalf("再送なのに Slack を叩いている: added=%v posted=%+v", api.added, api.posted)
	}
}

// 再送でない通常のイベントはこれまでどおり 202 で受け付けて非同期処理へ回す。
func TestWebhook_AcceptsFirstDelivery(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	bot := Bot{VerificationToken: "vt", SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}

	body := `{"token":"vt","event":{"type":"app_mention","text":"<@BOT> focus 8/8","channel":"C1","ts":"` + testMentionTS + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/slack/events", strings.NewReader(body))
	rec := httptest.NewRecorder()

	bot.Webhook(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	select {
	case <-enq.enqueue:
	case <-time.After(3 * time.Second):
		t.Fatal("enqueue されなかった")
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

// AC-5: echo は注入された ChatGPT を使う（都度クライアントを作らない）。
func TestEcho_UsesInjectedChatGPT(t *testing.T) {
	api := newFakeSlackAPI()
	gpt := &fakeChatGPT{reply: "こんにちは！"}
	bot := Bot{SlackAPI: api, ChatGPT: gpt, Enqueuer: newFakeEnqueuer()}

	bot.onMention(mentionPayload("<@BOT> 元気ですか", "C1", testMentionTS, ""))

	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	got := gpt.requests[0]
	if got.Model != chatModelLight {
		t.Fatalf("model = %q, want %s", got.Model, chatModelLight)
	}
	if len(got.System) != 6 {
		t.Fatalf("system = %d 本, want 6", len(got.System))
	}
	if !strings.Contains(got.System[2], BotAssistantName) {
		t.Fatalf("人格の指示が失われている: %q", got.System[2])
	}
	// AC-1: 名乗る名前は Slack App の実際の表示名と一致すること。
	// ここだけは定数ではなくリテラルで pin する（定数の書き換えを検知するため）。
	if !strings.Contains(got.System[2], "斧田 三葉") {
		t.Fatalf("system プロンプトが実際の bot 名を名乗っていない: %q", got.System[2])
	}
	if got.User != "元気ですか" {
		t.Fatalf("user = %q, want 元気ですか", got.User)
	}
	if got.Schema != nil {
		t.Fatal("echo に Structured Outputs は要らない")
	}
	if len(api.posted) != 1 || api.posted[0].Text() != "こんにちは！" {
		t.Fatalf("応答が投稿されていない: %+v", api.posted)
	}
}

// ChatGPT が無い環境（OPENAI_API_KEY 未設定）は従来どおりの定型文に落ちる。
func TestEcho_WithoutChatGPT(t *testing.T) {
	api := newFakeSlackAPI()
	bot := Bot{SlackAPI: api, Enqueuer: newFakeEnqueuer()}

	bot.onMention(mentionPayload("<@BOT> 元気ですか", "C1", testMentionTS, ""))

	if len(api.posted) != 1 || !strings.Contains(api.posted[0].Text(), "ちょっと何言っているかわからないです") {
		t.Fatalf("定型文に落ちていない: %+v", api.posted)
	}
}

// AC-5: 翻訳チャンネルへの投稿は軽量モデルで翻訳して相方チャンネルへ流す。
func TestOnMessage_Translate(t *testing.T) {
	api := newFakeSlackAPI()
	api.channelInfo = &slack.Channel{GroupConversation: slack.GroupConversation{
		Conversation: slack.Conversation{ID: "C1"}, Name: "team",
	}}
	api.conversations = []slack.Channel{{GroupConversation: slack.GroupConversation{
		Conversation: slack.Conversation{ID: "C2"}, Name: "team_fr",
	}}}
	gpt := &fakeChatGPT{reply: "Bonjour"}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	bot.onMessage(
		Payload{Event: map[string]any{"type": "message", "text": "おはよう", "channel": "C1", "ts": "100.000000"}})

	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	got := gpt.requests[0]
	if got.Model != chatModelLight {
		t.Fatalf("model = %q, want %s", got.Model, chatModelLight)
	}
	if len(got.System) != 1 || !strings.Contains(got.System[0], "translator") {
		t.Fatalf("system = %v", got.System)
	}
	if !strings.Contains(got.User, "おはよう") || !strings.Contains(got.User, "`fr`") {
		t.Fatalf("user = %q", got.User)
	}
	if len(api.posted) != 1 || api.posted[0].Channel != "C2" || api.posted[0].Text() != "Bonjour" {
		t.Fatalf("翻訳が相方チャンネルへ投稿されていない: %+v", api.posted)
	}
}

// LLM が使えない環境では翻訳を諦めるだけで、投稿もパニックもしない。
func TestOnMessage_WithoutChatGPT(t *testing.T) {
	api := newFakeSlackAPI()
	api.channelInfo = &slack.Channel{GroupConversation: slack.GroupConversation{
		Conversation: slack.Conversation{ID: "C1"}, Name: "team",
	}}
	api.conversations = []slack.Channel{{GroupConversation: slack.GroupConversation{
		Conversation: slack.Conversation{ID: "C2"}, Name: "team_fr",
	}}}
	bot := Bot{SlackAPI: api}

	bot.onMessage(
		Payload{Event: map[string]any{"type": "message", "text": "おはよう", "channel": "C1", "ts": "100.000000"}})

	if len(api.posted) != 0 {
		t.Fatalf("ChatGPT が無いのに投稿している: %+v", api.posted)
	}
}

// fakeEquipStore は LoadEquips に渡された context を記録するだけの EquipStore。
// #665 の「goroutine に cancel 済みの context が渡っていないか」を観測する。
type fakeEquipStore struct {
	gotCtx context.Context
	equips []models.Equip
	err    error
}

func (f *fakeEquipStore) LoadEquips(ctx context.Context) ([]models.Equip, error) {
	f.gotCtx = ctx
	return f.equips, f.err
}

// AC-2: onMentionEquipCheck は Webhook の goroutine から呼ばれるため、
// Datastore に渡る context が cancel されていてはならない。
func TestOnMentionEquipCheck_ContextNotCanceled(t *testing.T) {
	api := newFakeSlackAPI()
	store := &fakeEquipStore{}
	bot := Bot{SlackAPI: api, EquipStore: store, Enqueuer: newFakeEnqueuer()}

	bot.onMentionEquipCheck(mentionEvent("<@BOT> 備品", "C1", testMentionTS, ""))

	if store.gotCtx == nil {
		t.Fatal("EquipStore が呼ばれていない")
	}
	if err := store.gotCtx.Err(); err != nil {
		t.Fatalf("Datastore に cancel 済みの context が渡っている: %v", err)
	}
	if len(api.posted) != 1 {
		t.Fatalf("集計が投稿されていない: %+v", api.posted)
	}
}

// AC-2 の裏: 読み出しに失敗したら黙って return せずスレッドに理由を返す。
func TestOnMentionEquipCheck_PostsErrorToThread(t *testing.T) {
	api := newFakeSlackAPI()
	store := &fakeEquipStore{err: errStub("datastore down")}
	bot := Bot{SlackAPI: api, EquipStore: store, Enqueuer: newFakeEnqueuer()}

	bot.onMentionEquipCheck(mentionEvent("<@BOT> 備品", "C1", testMentionTS, testMentionTS))

	if len(api.posted) != 1 {
		t.Fatalf("失敗が黙って捨てられている: %+v", api.posted)
	}
	if !strings.Contains(api.posted[0].Text(), "datastore down") {
		t.Fatalf("失敗の理由がスレッドに出ていない: %q", api.posted[0].Text())
	}
	if api.posted[0].ThreadTS() != testMentionTS {
		t.Fatalf("スレッドに返していない: thread_ts=%q", api.posted[0].ThreadTS())
	}
}
