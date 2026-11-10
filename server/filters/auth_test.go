package filters

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

const (
	testSessionCookieName = "hub_session_test"
	testSigningKey        = "test-signing-key"
	testSlackID           = "U12345678"
)

// setupSessionEnv はセッション Cookie の検証に必要な設定をテスト用に差し替える。
func setupSessionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SIGNING_KEY", testSigningKey)
	original := server.SessionCookieName
	server.SessionCookieName = testSessionCookieName
	t.Cleanup(func() { server.SessionCookieName = original })
}

// rawToken は JWT ライブラリを通さずにトークンを組み立てる。
// signingKey が空なら署名部を空にする（alg: none 用）。
func rawToken(headerJSON, payloadJSON, signingKey string) string {
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString([]byte(headerJSON)) + "." + enc.EncodeToString([]byte(payloadJSON))
	if signingKey == "" {
		return signingInput + "."
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(signingInput))
	return signingInput + "." + enc.EncodeToString(mac.Sum(nil))
}

// signedToken は v5 ライブラリで HS256 署名した、ttl 後に期限が切れるトークンを返す。
func signedToken(t *testing.T, ttl time.Duration, slackID, signingKey string) string {
	t.Helper()
	claims := &models.SessionUserClaims{
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl))},
		SlackID:          slackID,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(signingKey))
	if err != nil {
		t.Fatalf("トークンの署名に失敗: %v", err)
	}
	return token
}

// legacyPayload は移行前（golang-jwt v3 の StandardClaims）が発行していた形式の、
// 期限内のペイロードを返す。
func legacyPayload() string {
	return fmt.Sprintf(`{"exp":%d,"SlackID":%q}`, time.Now().Add(24*time.Hour).Unix(), testSlackID)
}

type authResult struct {
	rec        *httptest.ResponseRecorder
	nextCalled bool
	slackID    string
}

// serveWithSessionCookie は token をセッション Cookie に入れて Auth を通す。
func serveWithSessionCookie(apiMode bool, token string) authResult {
	result := authResult{rec: httptest.NewRecorder()}
	next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		result.nextCalled = true
		result.slackID = GetSessionUserContext(req)
	})
	req := httptest.NewRequest(http.MethodGet, "/members?tab=1", nil)
	req.AddCookie(&http.Cookie{Name: testSessionCookieName, Value: token})
	(&Auth{API: apiMode}).Handle(next).ServeHTTP(result.rec, req)
	return result
}

// sessionCookieDeleted は応答がセッション Cookie を削除しているかを返す。
func sessionCookieDeleted(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == testSessionCookieName && c.Value == "" && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

// assertUnauthorized は、Cookie を消さずに 401 を返したことを確かめる。
func assertUnauthorized(t *testing.T, result authResult) {
	t.Helper()
	if result.nextCalled {
		t.Error("次のハンドラが呼ばれた")
	}
	if result.rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", result.rec.Code, http.StatusUnauthorized)
	}
	if sessionCookieDeleted(result.rec) {
		t.Error("セッション Cookie が削除された")
	}
}

// assertRedirectedToLogin は、Cookie を消してログインへ戻したことを確かめる。
func assertRedirectedToLogin(t *testing.T, result authResult) {
	t.Helper()
	if result.nextCalled {
		t.Error("次のハンドラが呼ばれた")
	}
	if result.rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", result.rec.Code, http.StatusTemporaryRedirect)
	}
	if got, want := result.rec.Header().Get("Location"), "/login?goto=%2Fmembers%3Ftab%3D1"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	if !sessionCookieDeleted(result.rec) {
		t.Error("セッション Cookie が削除されていない")
	}
}

// TestAuth_ValidSession は有効なセッション Cookie が通ることを固定する（Issue #724 AC-3, AC-7）。
func TestAuth_ValidSession(t *testing.T) {
	setupSessionEnv(t)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "v5 で発行したトークン",
			token: signedToken(t, time.Hour, testSlackID, testSigningKey),
		},
		{
			// ライブラリを通さずに組み立て、ログイン中の Cookie がそのまま通ることを確かめる。
			name:  "v3 が発行していた形式のトークン",
			token: rawToken(`{"alg":"HS256","typ":"JWT"}`, legacyPayload(), testSigningKey),
		},
	}
	for _, tt := range tests {
		for _, apiMode := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/api=%v", tt.name, apiMode), func(t *testing.T) {
				result := serveWithSessionCookie(apiMode, tt.token)
				if !result.nextCalled {
					t.Fatalf("次のハンドラが呼ばれなかった（status = %d）", result.rec.Code)
				}
				if result.slackID != testSlackID {
					t.Errorf("slackID = %q, want %q", result.slackID, testSlackID)
				}
			})
		}
	}
}

// TestAuth_InvalidToken は検証に失敗するトークンの拒否を固定する
// （Issue #724 AC-4, AC-5, AC-6, AC-13）。
// API モードは 401 のみで Cookie を消さず、ページモードは Cookie を消してログインへ戻す。
func TestAuth_InvalidToken(t *testing.T) {
	setupSessionEnv(t)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "期限切れ",
			token: signedToken(t, -time.Hour, testSlackID, testSigningKey),
		},
		{
			name:  "別の鍵で署名",
			token: signedToken(t, time.Hour, testSlackID, "another-signing-key"),
		},
		{
			name:  "alg: none",
			token: rawToken(`{"alg":"none","typ":"JWT"}`, legacyPayload(), ""),
		},
		{
			// 署名部は HMAC のまま、ヘッダだけ RSA を名乗るアルゴリズム混同。
			name:  "HMAC 以外の方式（RS256）",
			token: rawToken(`{"alg":"RS256","typ":"JWT"}`, legacyPayload(), testSigningKey),
		},
		{
			name:  "exp が無い",
			token: rawToken(`{"alg":"HS256","typ":"JWT"}`, fmt.Sprintf(`{"SlackID":%q}`, testSlackID), testSigningKey),
		},
		{
			name:  "JWT の形をしていない",
			token: "not-a-jwt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/api", func(t *testing.T) {
			assertUnauthorized(t, serveWithSessionCookie(true, tt.token))
		})
		t.Run(tt.name+"/page", func(t *testing.T) {
			assertRedirectedToLogin(t, serveWithSessionCookie(false, tt.token))
		})
	}
}

// TestAuth_EmptySlackID は、検証は通るが SlackID が空のトークンを
// 両モードでログインへ戻すことを固定する（Issue #724 AC-14）。
func TestAuth_EmptySlackID(t *testing.T) {
	setupSessionEnv(t)
	token := signedToken(t, time.Hour, "", testSigningKey)

	for _, apiMode := range []bool{true, false} {
		t.Run(fmt.Sprintf("api=%v", apiMode), func(t *testing.T) {
			assertRedirectedToLogin(t, serveWithSessionCookie(apiMode, token))
		})
	}
}
