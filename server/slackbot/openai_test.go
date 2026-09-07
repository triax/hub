package slackbot

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"
)

// chatStub は OpenAI の /chat/completions を模し、受け取ったリクエストボディを
// 捕捉して固定の応答を返す。実 API は叩かない。
type chatStub struct {
	server   *httptest.Server
	body     map[string]any
	rawBody  string
	authz    string
	response string
}

func newChatStub(t *testing.T, response string) *chatStub {
	t.Helper()
	stub := &chatStub{response: response}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		stub.rawBody = string(buf)
		stub.authz = r.Header.Get("Authorization")
		if err := json.Unmarshal(buf, &stub.body); err != nil {
			t.Errorf("unmarshal request body: %v (%s)", err, stub.rawBody)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(stub.response))
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *chatStub) chat(t *testing.T) *openAIChat {
	t.Helper()
	return newOpenAIChat(option.WithAPIKey("test-key"), option.WithBaseURL(s.server.URL))
}

func okResponse(content string) string {
	quoted, _ := json.Marshal(content)
	return `{"id":"x","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":` +
		string(quoted) + `}}]}`
}

// AC-1: Schema 付きの ChatRequest は response_format に strict な json_schema を載せる。
func TestOpenAIChat_StructuredOutputs(t *testing.T) {
	stub := newChatStub(t, okResponse(`{"focus":[]}`))

	got, err := stub.chat(t).Chat(t.Context(), ChatRequest{
		Model:  chatModelFocus,
		System: []string{"prompt"},
		User:   "input",
		Schema: &ChatJSONSchema{Name: focusDigestSchemaName, Schema: focusDigestSchema},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != `{"focus":[]}` {
		t.Fatalf("content = %q", got)
	}
	if !strings.Contains(stub.authz, "test-key") {
		t.Fatalf("Authorization = %q, want Bearer test-key", stub.authz)
	}
	if stub.body["model"] != "gpt-4o" {
		t.Fatalf("model = %v, want gpt-4o", stub.body["model"])
	}

	format, ok := stub.body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format が無い: %s", stub.rawBody)
	}
	if format["type"] != "json_schema" {
		t.Fatalf("response_format.type = %v, want json_schema", format["type"])
	}
	schema, ok := format["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema が無い: %s", stub.rawBody)
	}
	if schema["name"] != focusDigestSchemaName {
		t.Fatalf("json_schema.name = %v, want %s", schema["name"], focusDigestSchemaName)
	}
	if schema["strict"] != true {
		t.Fatalf("json_schema.strict = %v, want true", schema["strict"])
	}

	// 送った schema がそのまま届いていること（JSON 経由で正規化して比較する）。
	want := map[string]any{}
	raw, _ := json.Marshal(focusDigestSchema)
	json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(schema["schema"], want) {
		t.Fatalf("json_schema.schema が focusDigestSchema と一致しない:\n got: %v\nwant: %v", schema["schema"], want)
	}
}

// AC-2: Schema なしなら response_format を付けず、messages は system…→ user の順。
func TestOpenAIChat_NoSchema(t *testing.T) {
	stub := newChatStub(t, okResponse("bonjour"))

	got, err := stub.chat(t).Chat(t.Context(), ChatRequest{
		Model:  chatModelLight,
		System: []string{"s1", "s2"},
		User:   "u",
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "bonjour" {
		t.Fatalf("content = %q, want bonjour", got)
	}
	if _, ok := stub.body["response_format"]; ok {
		t.Fatalf("Schema なしなのに response_format が付いている: %s", stub.rawBody)
	}
	if stub.body["model"] != chatModelLight {
		t.Fatalf("model = %v, want %s", stub.body["model"], chatModelLight)
	}

	messages, ok := stub.body["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("messages = %v, want 3 件", stub.body["messages"])
	}
	want := []struct{ role, content string }{{"system", "s1"}, {"system", "s2"}, {"user", "u"}}
	for i, w := range want {
		m := messages[i].(map[string]any)
		if m["role"] != w.role || m["content"] != w.content {
			t.Fatalf("messages[%d] = %v, want role=%s content=%s", i, m, w.role, w.content)
		}
	}
}

// AC-3: refusal と choices 空はエラーにする（黙って空文字を返さない）。
func TestOpenAIChat_RefusalAndEmptyChoices(t *testing.T) {
	t.Run("refusal", func(t *testing.T) {
		stub := newChatStub(t, `{"id":"x","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"","refusal":"お答えできません"}}]}`)
		_, err := stub.chat(t).Chat(t.Context(), ChatRequest{Model: chatModelFocus, User: "u"})
		if err == nil || !strings.Contains(err.Error(), "お答えできません") {
			t.Fatalf("refusal が error になっていない: %v", err)
		}
	})

	t.Run("choices 空", func(t *testing.T) {
		stub := newChatStub(t, `{"id":"x","object":"chat.completion","model":"gpt-4o","choices":[]}`)
		_, err := stub.chat(t).Chat(t.Context(), ChatRequest{Model: chatModelFocus, User: "u"})
		if err == nil {
			t.Fatal("choices 空が error になっていない")
		}
	})
}

// キーが無い環境では nil を返し、呼び出し側の「LLM 無し」分岐に落とす。
func TestNewOpenAIChat_EmptyKey(t *testing.T) {
	if got := NewOpenAIChat(""); got != nil {
		t.Fatalf("NewOpenAIChat(\"\") = %v, want nil", got)
	}
	if got := NewOpenAIChat("sk-test"); got == nil {
		t.Fatal("キーがあるのに nil が返っている")
	}
}
