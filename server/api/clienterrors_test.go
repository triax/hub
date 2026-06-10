package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/triax/hub/server/filters"
)

// TestReportClientError_Valid は、正しい body を送ると 204 を返すことを検証する。
// SLACK_CHANNEL_ALERTS 未設定の環境では observability.Notify は no-op なので Slack へは投稿されない。
func TestReportClientError_Valid(t *testing.T) {
	body := `{"message":"Minified React error #130","stack":"at RSVPModal","url":"https://hub.triax.football/events/x","ts":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/1/client-errors", strings.NewReader(body))
	// /api/1/* は認証下に置くため、セッションユーザを付与した状態を再現する。
	req = filters.SetSessionUserContext(req, "U12345678")
	rec := httptest.NewRecorder()

	ReportClientError(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

// TestReportClientError_MissingMessage は、message 欠落時に 400 を返すことを検証する。
func TestReportClientError_MissingMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/1/client-errors", strings.NewReader(`{"stack":"x"}`))
	req = filters.SetSessionUserContext(req, "U12345678")
	rec := httptest.NewRecorder()

	ReportClientError(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestReportClientError_InvalidJSON は、壊れた JSON で 400 を返すことを検証する。
func TestReportClientError_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/1/client-errors", strings.NewReader(`{not json`))
	req = filters.SetSessionUserContext(req, "U12345678")
	rec := httptest.NewRecorder()

	ReportClientError(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
