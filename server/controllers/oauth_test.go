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

	token, err := newSessionToken("U12345678")
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}

	for _, apiMode := range []bool{true, false} {
		var gotSlackID string
		next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			gotSlackID = filters.GetSessionUserContext(req)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: server.SessionCookieName, Value: token})
		rec := httptest.NewRecorder()

		(&filters.Auth{API: apiMode}).Handle(next).ServeHTTP(rec, req)

		if gotSlackID != "U12345678" {
			t.Errorf("api=%v: slackID = %q, want %q（status = %d）", apiMode, gotSlackID, "U12345678", rec.Code)
		}
	}
}
