package filters

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/otiai10/marmoset"
	"github.com/triax/hub/server/observability"
)

// Recovery は panic を捕捉し、500 を返しつつ Slack へアラートを送る middleware。
// 本番でハンドラが panic してもプロセスを落とさず、可観測性を確保する。
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := string(debug.Stack())
				log.Printf("[ERROR] 10001 panic recovered: %v\n%s", rec, stack)
				observability.Notify(observability.Report{
					Source:  "backend/panic",
					Message: fmt.Sprintf("%v", rec),
					Stack:   stack,
					URL:     r.Method + " " + r.URL.Path,
					User:    sessionUserSafe(r),
				})
				marmoset.Render(w).JSON(http.StatusInternalServerError, marmoset.P{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// sessionUserSafe はセッションユーザ ID を panic せずに取得する。
// Recovery は認証前のルートも含めて全リクエストを包むため、
// セッション未設定でも安全に空文字を返す必要がある。
func sessionUserSafe(r *http.Request) string {
	if v, ok := r.Context().Value(SessionContextKey).(string); ok {
		return v
	}
	return ""
}
