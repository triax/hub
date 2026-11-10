package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/triax/hub/server"
	"github.com/triax/hub/server/filters"
)

// TestNewSessionToken_AcceptedByAuthFilter は、ログイン処理が発行するトークンを
// 認証フィルタがそのまま受け入れることを固定する（Issue #724 AC-12）。
func TestNewSessionToken_AcceptedByAuthFilter(t *testing.T) {
	t.Setenv("JWT_SIGNING_METHOD", "HS256")
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	original := server.SessionCookieName
	server.SessionCookieName = "hub_session_test"
	t.Cleanup(func() { server.SessionCookieName = original })

	const slackID = "U12345678"
	token, err := newSessionToken(slackID)
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}

	// 拒否時のモード別の応答は filters 側のテストが固定する。ここは発行と検証の一致だけを見る。
	var gotSlackID string
	next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotSlackID = filters.GetSessionUserContext(req)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: server.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()

	(&filters.Auth{API: true}).Handle(next).ServeHTTP(rec, req)

	if gotSlackID != slackID {
		t.Errorf("slackID = %q, want %q（status = %d）", gotSlackID, slackID, rec.Code)
	}
}
