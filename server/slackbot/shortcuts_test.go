package slackbot

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

// AC-5: message shortcut の翻訳も注入クライアント経由・軽量モデルで動く。
func TestTranslate(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &got)
	}))
	defer srv.Close()

	gpt := &fakeChatGPT{reply: "Good morning"}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
	payload := slack.InteractionCallback{ResponseURL: srv.URL}
	payload.Message.Text = "おはよう"

	if err := bot.Translate(t.Context(), payload, "en"); err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	req := gpt.requests[0]
	if req.Model != chatModelLight {
		t.Fatalf("model = %q, want %s", req.Model, chatModelLight)
	}
	if len(req.System) != 1 || !strings.Contains(req.System[0], "translator") {
		t.Fatalf("system = %v", req.System)
	}
	if !strings.Contains(req.User, "おはよう") || !strings.Contains(req.User, "`en`") {
		t.Fatalf("user = %q", req.User)
	}
	if got["text"] != "Good morning" {
		t.Fatalf("response_url に返した本文 = %q, want Good morning", got["text"])
	}
}

// LLM が使えない環境では ErrNoChatGPT を返す（nil 参照で落ちない）。
func TestTranslate_WithoutChatGPT(t *testing.T) {
	bot := Bot{SlackAPI: newFakeSlackAPI()}
	err := bot.Translate(t.Context(), slack.InteractionCallback{}, "en")
	if !errors.Is(err, ErrNoChatGPT) {
		t.Fatalf("err = %v, want ErrNoChatGPT", err)
	}
}
