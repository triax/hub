package filters

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParsePublicAPIKeys は PUBLIC_API_KEYS のパース規則を固定する（Issue #645 AC-1）。
func TestParsePublicAPIKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "複数エントリ",
			input: "homepage:aaa,insta:bbb",
			want:  map[string]string{"homepage": "aaa", "insta": "bbb"},
		},
		{
			name:  "空文字列は空 map",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "前後の空白は無視する",
			input: "  homepage : aaa , insta:bbb  ",
			want:  map[string]string{"homepage": "aaa", "insta": "bbb"},
		},
		{
			name:  "コロンを含まないエントリは無視する",
			input: "homepage:aaa,broken,insta:bbb",
			want:  map[string]string{"homepage": "aaa", "insta": "bbb"},
		},
		{
			name:  "name が空のエントリは無視する",
			input: ":aaa,insta:bbb",
			want:  map[string]string{"insta": "bbb"},
		},
		{
			name:  "key が空のエントリは無視する",
			input: "homepage:,insta:bbb",
			want:  map[string]string{"insta": "bbb"},
		},
		{
			name:  "key 側のコロンは保全する",
			input: "homepage:aa:bb",
			want:  map[string]string{"homepage": "aa:bb"},
		},
		{
			name:  "同名は後勝ち",
			input: "homepage:old,homepage:new",
			want:  map[string]string{"homepage": "new"},
		},
		{
			name:  "区切りのみの文字列は空 map",
			input: ",,",
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePublicAPIKeys(tt.input)
			if got == nil {
				t.Fatal("parsePublicAPIKeys must return an empty map, not nil")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parsePublicAPIKeys(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for name, key := range tt.want {
				if got[name] != key {
					t.Fatalf("parsePublicAPIKeys(%q) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}

// TestAuthorizePublicAPIKey は照合規則を固定する（Issue #645 AC-2 / AC-3）。
func TestAuthorizePublicAPIKey(t *testing.T) {
	keys := map[string]string{"homepage": "aaa", "insta": "bbb"}

	tests := []struct {
		name      string
		keys      map[string]string
		presented string
		wantName  string
		wantOK    bool
	}{
		{name: "1 つ目のキーで一致", keys: keys, presented: "aaa", wantName: "homepage", wantOK: true},
		{name: "2 つ目のキーで一致", keys: keys, presented: "bbb", wantName: "insta", wantOK: true},
		{name: "不一致", keys: keys, presented: "zzz"},
		{name: "前方一致は通さない", keys: keys, presented: "aa"},
		{name: "提示が空文字", keys: keys, presented: ""},
		{name: "設定が空でも空文字は通さない", keys: map[string]string{}, presented: ""},
		{name: "設定が空なら正しそうなキーでも拒否", keys: map[string]string{}, presented: "aaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, ok := authorizePublicAPIKey(tt.keys, tt.presented)
			if ok != tt.wantOK || name != tt.wantName {
				t.Fatalf("authorizePublicAPIKey(%v, %q) = (%q, %v), want (%q, %v)",
					tt.keys, tt.presented, name, ok, tt.wantName, tt.wantOK)
			}
		})
	}
}

// serveWithKey は PUBLIC_API_KEYS を env に立てて middleware を 1 回実行し、
// レスポンスと「次のハンドラが呼ばれたか」を返すヘルパ。
func serveWithKey(t *testing.T, env, header string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	t.Setenv(PublicAPIKeysEnv, env)

	reached := false
	h := RequirePublicAPIKey(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/1/public/members", nil)
	if header != "" {
		req.Header.Set(PublicAPIKeyHeader, header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, reached
}

// TestRequirePublicAPIKey は middleware の許可・拒否を固定する（Issue #645 AC-2）。
func TestRequirePublicAPIKey(t *testing.T) {
	const env = "homepage:aaa,insta:bbb"

	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantNext   bool
	}{
		{name: "ヘッダなしは 401", header: "", wantStatus: http.StatusUnauthorized},
		{name: "不一致キーは 401", header: "zzz", wantStatus: http.StatusUnauthorized},
		{name: "正しいキーは通す", header: "aaa", wantStatus: http.StatusOK, wantNext: true},
		{name: "別エントリのキーでも通す", header: "bbb", wantStatus: http.StatusOK, wantNext: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, reached := serveWithKey(t, env, tt.header)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if reached != tt.wantNext {
				t.Fatalf("next handler reached = %v, want %v", reached, tt.wantNext)
			}
		})
	}
}

// TestRequirePublicAPIKey_UnauthorizedBody は 401 のボディ形状を固定する。
// 既存 API と同じ {"error": ...} の JSON にそろえ、キー値は一切含めない。
func TestRequirePublicAPIKey_UnauthorizedBody(t *testing.T) {
	rec, _ := serveWithKey(t, "homepage:aaa", "wrong-key")

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("401 body must be JSON: %v (body=%q)", err, rec.Body.String())
	}
	if body["error"] != "unauthorized" {
		t.Fatalf(`body["error"] = %v, want "unauthorized"`, body["error"])
	}
	if got := rec.Body.String(); strings.Contains(got, "wrong-key") || strings.Contains(got, "aaa") {
		t.Fatalf("401 body must not leak key material: %q", got)
	}
}

// TestRequirePublicAPIKey_DenyByDefault は全拒否既定を固定する（Issue #645 AC-3）。
// PUBLIC_API_KEYS が未設定・空・不正のみのとき、どんなキーを付けても通らない。
func TestRequirePublicAPIKey_DenyByDefault(t *testing.T) {
	envs := map[string]string{
		"空文字列":      "",
		"区切りのみ":     ",,",
		"コロンなしの不正値": "homepage",
		"key が空":    "homepage:",
	}
	headers := map[string]string{
		"ヘッダなし":   "",
		"それらしいキー": "aaa",
		"空ヘッダ":    " ",
	}

	for envName, env := range envs {
		for headerName, header := range headers {
			t.Run(envName+"/"+headerName, func(t *testing.T) {
				rec, reached := serveWithKey(t, env, header)
				if rec.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
				}
				if reached {
					t.Fatal("next handler must not be reached when no key is configured")
				}
			})
		}
	}
}
