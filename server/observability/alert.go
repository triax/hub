// Package observability はフロント／バックエンドで発生したエラーを
// 登録された Slack チャンネルへアラート投稿するためのヘルパーを提供する。
//
// 通知先チャンネルは環境変数 SLACK_CHANNEL_ALERTS で解決する。
// GAE は dev/prod でデプロイ環境（env_variables）が分かれるため、
// この 1 変数だけで環境ごとの通知先切り替えが成立する。
package observability

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

// alertChannelEnv は通知先チャンネル ID を解決する環境変数名。
const alertChannelEnv = "SLACK_CHANNEL_ALERTS"

// dedupWindow は同一エラーの連投を抑制する時間窓。
const dedupWindow = 60 * time.Second

// maxStackLen は Slack のテキストブロック上限（3000字）に収めるためのスタック切り詰め長。
const maxStackLen = 2600

// Report は 1 件のエラーアラートに含める情報。任意項目は空でよい。
type Report struct {
	Source         string    // 発生元（例: "frontend", "backend/panic"）
	Message        string    // エラーメッセージ
	Stack          string    // スタックトレース（JS の minified stack / Go の debug.Stack）
	ComponentStack string    // React コンポーネントスタック（描画クラッシュの犯人特定に有効）
	URL            string    // 発生 URL / エンドポイント
	Referer        string    // 遷移元（任意）
	UserAgent      string    // クライアント UserAgent
	Release        string    // ビルド識別子
	User           string    // Slack ユーザ ID 等
	Time           time.Time // 発生時刻（zero 値なら送信時刻で補完）
}

// 同一エラーの短時間連投を抑制するための簡易 dedup 状態。
var (
	dedupMu       sync.Mutex
	dedupLastSent = map[string]time.Time{}
)

// Notify はエラーアラートを Slack へ非同期投稿する fire-and-forget なヘルパー。
//
// アラート送信自体が呼び出し元（recovery middleware 等）を二次破壊しないよう、
// 内部で goroutine + recover ガードする。通知先チャンネル（SLACK_CHANNEL_ALERTS）が
// 未設定の環境（ローカル開発・テスト等）では no-op となる。
func Notify(r Report) {
	channel := os.Getenv(alertChannelEnv)
	if channel == "" {
		log.Printf("[observability] %s 未設定のためアラートを送信しません: %s", alertChannelEnv, r.Message)
		return
	}
	if r.Time.IsZero() {
		r.Time = time.Now()
	}
	if suppressed(r) {
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[observability] アラート送信中に panic: %v", rec)
			}
		}()
		api := slack.New(os.Getenv("SLACK_BOT_USER_OAUTH_TOKEN"))
		if _, _, err := api.PostMessage(channel, slack.MsgOptionBlocks(r.blocks()...)); err != nil {
			log.Printf("[observability] アラート投稿に失敗: %v", err)
		}
	}()
}

// suppressed は直近 dedupWindow 内に同一（発生元＋メッセージ）のアラートを
// 送っていれば true を返し、送信を抑制する。
func suppressed(r Report) bool {
	key := r.Source + "|" + r.Message
	dedupMu.Lock()
	defer dedupMu.Unlock()
	now := time.Now()
	if last, ok := dedupLastSent[key]; ok && now.Sub(last) < dedupWindow {
		return true
	}
	dedupLastSent[key] = now
	return false
}

// envLabel はアラートに表示するデプロイ環境ラベルを返す。
// GAE では GOOGLE_CLOUD_PROJECT がプロジェクト（dev/prod）を表す。
func envLabel() string {
	if p := os.Getenv("GOOGLE_CLOUD_PROJECT"); p != "" {
		return p
	}
	return "local"
}

// blocks は Report を原因特定に十分な情報を含む Slack Block Kit へ整形する。
func (r Report) blocks() []slack.Block {
	header := slack.NewHeaderBlock(
		slack.NewTextBlockObject(slack.PlainTextType, "🚨 エラー検知", true, false),
	)

	fields := []*slack.TextBlockObject{
		slack.NewTextBlockObject(slack.MarkdownType, "*環境:*\n"+envLabel(), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, "*発生元:*\n"+orDash(r.Source), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, "*時刻:*\n"+r.Time.Format(time.RFC3339), false, false),
		slack.NewTextBlockObject(slack.MarkdownType, "*ユーザ:*\n"+orDash(r.User), false, false),
	}
	meta := slack.NewSectionBlock(nil, fields, nil)

	body := fmt.Sprintf("*メッセージ:*\n```%s```", truncate(orDash(r.Message), maxStackLen))
	if r.URL != "" {
		body += "\n*発生箇所:* `" + r.URL + "`"
	}
	if r.Referer != "" {
		body += "\n*遷移元:* `" + r.Referer + "`"
	}
	if r.Release != "" {
		body += "\n*リリース:* `" + r.Release + "`"
	}
	if r.UserAgent != "" {
		body += "\n*UA:* " + r.UserAgent
	}
	msg := slack.NewSectionBlock(
		slack.NewTextBlockObject(slack.MarkdownType, body, false, false), nil, nil,
	)

	blocks := []slack.Block{header, meta, msg}
	// React コンポーネントスタック（描画クラッシュの犯人特定に最も有効）
	if strings.TrimSpace(r.ComponentStack) != "" {
		cs := slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*コンポーネントスタック:*\n```"+truncate(r.ComponentStack, maxStackLen)+"```", false, false),
			nil, nil,
		)
		blocks = append(blocks, cs)
	}
	if strings.TrimSpace(r.Stack) != "" {
		stack := slack.NewSectionBlock(
			slack.NewTextBlockObject(slack.MarkdownType, "*スタック:*\n```"+truncate(r.Stack, maxStackLen)+"```", false, false),
			nil, nil,
		)
		blocks = append(blocks, stack)
	}
	return blocks
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…(truncated)"
}
