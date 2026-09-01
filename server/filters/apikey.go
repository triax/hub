package filters

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/otiai10/marmoset"
)

const (
	// PublicAPIKeysEnv は名前付き共有キーを保持する環境変数名。
	// 形式は "name:key,name2:key2"（例: "homepage:xxx,instagram:yyy"）。
	// 消費者ごとに名前を持たせることで、識別・個別失効・無停止ローテーション
	// （新旧併記 → 消費者切替 → 旧削除）をこの 1 変数で賄う。
	PublicAPIKeysEnv = "PUBLIC_API_KEYS"

	// PublicAPIKeyHeader は公開 API のキーを受け取るリクエストヘッダ名。
	// クエリパラメータはアクセスログや Referer 経由で漏れるため使わない。
	PublicAPIKeyHeader = "X-API-Key"
)

// parsePublicAPIKeys は "name:key,name2:key2" を map[name]key にパースする。
//
//   - 各エントリの前後空白は無視する
//   - ":" を含まない・name が空・key が空 のエントリは無視する
//   - key 側に ":" があっても壊れないよう、最初の ":" のみで分割する
//   - 同名が重複した場合は後勝ち
//
// キーは openssl rand -hex 32 での生成を案内しているため、区切り文字（"," / ":"）が
// キー値に現れることは想定しない（エスケープ構文は持たない）。
func parsePublicAPIKeys(s string) map[string]string {
	keys := map[string]string{}
	for _, entry := range strings.Split(s, ",") {
		name, key, found := strings.Cut(strings.TrimSpace(entry), ":")
		if !found {
			continue
		}
		name, key = strings.TrimSpace(name), strings.TrimSpace(key)
		if name == "" || key == "" {
			continue
		}
		keys[name] = key
	}
	return keys
}

// authorizePublicAPIKey は提示されたキーを全エントリと照合し、一致した消費者名を返す。
//
// 比較は crypto/subtle.ConstantTimeCompare を使い、一致しても走査を打ち切らない
// （どのエントリで一致したかを応答時間から推測されないようにするため）。
// 提示が空文字のときは、空の設定値との「一致」が起きないよう即座に拒否する。
func authorizePublicAPIKey(keys map[string]string, presented string) (string, bool) {
	if presented == "" {
		return "", false
	}
	matched := ""
	for name, key := range keys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(presented)) == 1 {
			matched = name
		}
	}
	return matched, matched != ""
}

// RequirePublicAPIKey は X-API-Key ヘッダを検証する middleware。
//
// 全拒否既定: ヘッダ欠落・不一致・PUBLIC_API_KEYS 未設定/空 のいずれも 401 を返す。
// 設定ミスで公開 API が静かに素通しになる状態を作らないため、
// 「キーが無い環境では誰も通れない」側に倒している。
//
// 環境変数は Auth（JWT_SIGNING_KEY）や RequireGAECron と同じく使用時に読む。
func RequirePublicAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		keys := parsePublicAPIKeys(os.Getenv(PublicAPIKeysEnv))
		name, ok := authorizePublicAPIKey(keys, req.Header.Get(PublicAPIKeyHeader))
		if !ok {
			// 提示されたキーはログに出さない（失敗ログを秘密の保管庫にしないため）。
			marmoset.Render(w).JSON(http.StatusUnauthorized, marmoset.P{"error": "unauthorized"})
			return
		}
		// 消費者名だけを記録する。キー値そのものは決してログへ出さない。
		log.Printf("[INFO] public api access: consumer=%s %s %s", name, req.Method, req.URL.Path)
		next.ServeHTTP(w, req)
	})
}
