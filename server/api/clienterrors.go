package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/filters"
	"github.com/triax/hub/server/observability"
)

// ReportClientError はフロントエンドからのエラー報告を受け取り、
// 登録された Slack チャンネルへアラートを送る。レスポンスは 204 No Content。
//
// /api/1/* 配下（JWT 認証）に置くため、報告には呼び出しユーザの Slack ID を付与する。
func ReportClientError(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w)

	var input struct {
		Message   string `json:"message"`
		Stack     string `json:"stack"`
		URL       string `json:"url"`
		UserAgent string `json:"userAgent"`
		Release   string `json:"release"`
		TS        int64  `json:"ts"` // epoch ミリ秒
	}
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": err.Error()})
		return
	}
	if input.Message == "" {
		render.JSON(http.StatusBadRequest, marmoset.P{"error": "message is required"})
		return
	}

	report := observability.Report{
		Source:    "frontend",
		Message:   input.Message,
		Stack:     input.Stack,
		URL:       input.URL,
		UserAgent: input.UserAgent,
		Release:   input.Release,
		User:      filters.GetSessionUserContext(req),
	}
	if input.TS > 0 {
		report.Time = time.UnixMilli(input.TS)
	}
	observability.Notify(report)

	w.WriteHeader(http.StatusNoContent)
}
