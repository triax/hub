package filters

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecovery_RecoversPanic は、ハンドラが panic しても
// middleware が 500 を返し、panic を呼び出し元へ伝播しないことを検証する。
// SLACK_CHANNEL_ALERTS 未設定の環境では observability.Notify は no-op なので副作用は発生しない。
func TestRecovery_RecoversPanic(t *testing.T) {
	h := Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/1/explode", nil)
	rec := httptest.NewRecorder()

	// panic が伝播するとこの呼び出しが落ちる（テスト失敗）。
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestRecovery_PassThrough は、panic しない通常のレスポンスをそのまま通すことを検証する。
func TestRecovery_PassThrough(t *testing.T) {
	h := Recovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "ok")
	}
}
