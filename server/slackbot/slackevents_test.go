package slackbot

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
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

// unsetOpenAIKey は echo が実 OpenAI を叩かないようにする。
func unsetOpenAIKey(t *testing.T) {
	t.Helper()
	if v, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
		os.Unsetenv("OPENAI_API_KEY")
		t.Cleanup(func() { os.Setenv("OPENAI_API_KEY", v) })
	}
}

// AC-5: `focus` 以外のトークンで focus が起動しないこと（既存の分岐が変わらないこと）。
//
// `備品` は Datastore へ接続するため、ここでは対象外にしている
// （本番プロジェクトへ実アクセスしうるので自動テストでは踏まない）。
func TestOnMention_Dispatch(t *testing.T) {
	unsetOpenAIKey(t)

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

			req := httptest.NewRequest(http.MethodPost, "/slack/events", nil)
			bot.onMention(req, httptest.NewRecorder(), mentionPayload(c.text, "C1", testMentionTS, ""))

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
