package filters

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// AC-7: RequireGAECron は GAE cron と Cloud Tasks の両方を通し、それ以外を 403 で弾く。
// どちらのヘッダーも App Engine が外部からのリクエストで剥ぎ取るため詐称できない。
func TestRequireGAECron(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"GAE cron", map[string]string{"X-Appengine-Cron": "true"}, http.StatusOK},
		{"Cloud Tasks", map[string]string{"X-AppEngine-QueueName": "focus"}, http.StatusOK},
		{"両方", map[string]string{"X-Appengine-Cron": "true", "X-AppEngine-QueueName": "focus"}, http.StatusOK},
		{"ヘッダーなし", nil, http.StatusForbidden},
		{"cron が true でない", map[string]string{"X-Appengine-Cron": "false"}, http.StatusForbidden},
		{"QueueName が空", map[string]string{"X-AppEngine-QueueName": ""}, http.StatusForbidden},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			h := RequireGAECron(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/tasks/focus", nil)
			for k, v := range c.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d", rec.Code, c.want)
			}
			if called != (c.want == http.StatusOK) {
				t.Fatalf("next handler called = %v, want %v", called, c.want == http.StatusOK)
			}
		})
	}
}
