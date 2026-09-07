package filters

import "net/http"

// SecurityHeaders はセキュリティ関連のHTTPヘッダーを設定するミドルウェア
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// RequireGAECron はGoogle App Engineのcron/Cloud Tasksリクエストのみ許可するミドルウェア。
// どちらのヘッダーもApp Engineが外部からのリクエストで必ず剥ぎ取るため、詐称できない。
func RequireGAECron(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isCron := r.Header.Get("X-Appengine-Cron") == "true"
		isTask := r.Header.Get("X-AppEngine-QueueName") != ""
		if !isCron && !isTask {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MaxBodySize はリクエストボディのサイズを制限するミドルウェア
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
