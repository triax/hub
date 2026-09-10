package slackbot

import (
	"fmt"
	"log"

	"github.com/slack-go/slack"
)

// premortemSink は premortem の出力先。mention（`@<bot> premortem`）はチャンネルへ
// スレッド + broadcast で流し、slash（`/premortem`）は打った人だけに見える ephemeral へ流す。
//
// 出力は 7 箇所に分かれていて、モードごとに全部が変わる。`premortem()` の中に
// `if job.Ephemeral` を撒くのではなく**出力先そのものを差し替える**ことで、
// どちらのモードで何が起きるかがこのファイル 1 枚で読める（#695）。
//
// ephemeral には構造上できないことがある。**編集できない**（`chat.update` の対象外）ので
// 進捗更新と完了メタは出せず、**リアクションを付ける先のメッセージが無い**ので 👀 / ✅ も
// 付けられない。それらは no-op にして、呼び出し側から分岐を消す。
type premortemSink interface {
	// Working は「受け付けた」合図。mention は 👀、ephemeral は何もしない
	// （受付メッセージそのものが合図になる）。
	Working()
	// Receipt は受付メッセージ。以降の Progress はこれを書き換える（ephemeral は書き換えない）。
	Receipt(text string) error
	// Progress は収集の進捗。ephemeral は編集できないので no-op。
	Progress(done, total int)
	// Notice は警告・エラー。読み手に必ず届ける（黙って失敗しない）。
	Notice(text string)
	// Deliver は結果本体。
	Deliver(msgs []premortemMessage) error
	// Finish は完了メタ（期間・件数・所要時間）。ephemeral は編集できないので no-op。
	Finish(text string)
	// Close は後始末。mention は 👀 を外し、成功なら ✅ を付ける。ephemeral は no-op。
	Close(ok bool)
}

// premortemSinkFor は job の配送指定から出力先を組む。
func (bot Bot) premortemSinkFor(job premortemJob) premortemSink {
	if job.Ephemeral {
		return &ephemeralSink{bot: bot, job: job}
	}
	return &channelSink{bot: bot, job: job}
}

// ---------------------------------------------------------------- channel ---

// channelSink は従来どおりチャンネルへ流す。起動したメッセージ（mention 本体）の ts に
// ぶら下げ、1 通目だけ broadcast する。
type channelSink struct {
	bot      Bot
	job      premortemJob
	statusTS string // Receipt が返す ts。Progress / Finish が書き換える
}

func (s *channelSink) base() focusJob { return s.job.focusJobFor(s.job.Channel) }

func (s *channelSink) mention() slack.ItemRef {
	return slack.NewRefToMessage(s.job.Channel, s.job.MentionTS)
}

func (s *channelSink) Working() {
	_ = callSlack(func() error { return s.bot.SlackAPI.AddReaction(focusReactionWorking, s.mention()) })
}

func (s *channelSink) Receipt(text string) error {
	ts, err := s.bot.postStatus(s.base(), text)
	s.statusTS = ts
	return err
}

func (s *channelSink) Progress(done, total int) {
	if update := s.bot.progressUpdater(s.base(), s.statusTS); update != nil {
		update(done, total)
	}
}

func (s *channelSink) Notice(text string) {
	_ = s.bot.replyToMention(s.base(), text)
}

func (s *channelSink) Deliver(msgs []premortemMessage) error {
	return s.bot.postThreadMessages(s.job.Channel, s.job.MentionTS, s.job.ThreadOnly, msgs)
}

func (s *channelSink) Finish(text string) {
	if s.statusTS == "" {
		return
	}
	_ = callSlack(func() error {
		_, _, _, err := s.bot.SlackAPI.UpdateMessage(s.job.Channel, s.statusTS, slack.MsgOptionText(text, false))
		return err
	})
}

func (s *channelSink) Close(ok bool) {
	_ = callSlack(func() error { return s.bot.SlackAPI.RemoveReaction(focusReactionWorking, s.mention()) })
	if ok {
		_ = callSlack(func() error { return s.bot.SlackAPI.AddReaction(focusReactionDone, s.mention()) })
	}
}

// -------------------------------------------------------------- ephemeral ---

// ephemeralSink は打った人だけに見える形で流す。チャンネルには何も残らない。
//
// chat.postEphemeral を使う（response_url ではなく）。ワーカーは Cloud Tasks 経由なので、
// キューが滞留すると response_url は 30 分で黙って失効する。postEphemeral に期限は無い。
// bot がチャンネルに参加している必要はあるが、収集（conversations.history）が
// どのみち参加を要求するので実質的な追加制約にならない。
type ephemeralSink struct {
	bot Bot
	job premortemJob
}

func (s *ephemeralSink) post(opts ...slack.MsgOption) error {
	return callSlack(func() error {
		_, err := s.bot.SlackAPI.PostEphemeral(s.job.Channel, s.job.UserID, opts...)
		return err
	})
}

// 👀 を付ける先のメッセージが無い。受付メッセージ自体が「受け付けた」合図になる。
func (s *ephemeralSink) Working() {}

func (s *ephemeralSink) Receipt(text string) error {
	return s.post(slack.MsgOptionText(text, false))
}

// ephemeral は編集できない（chat.update の対象外）。進捗は出せない。
func (s *ephemeralSink) Progress(done, total int) {}

func (s *ephemeralSink) Notice(text string) {
	if err := s.post(slack.MsgOptionText(text, false)); err != nil {
		log.Println("[premortem] ephemeral notice:", err)
	}
}

func (s *ephemeralSink) Deliver(msgs []premortemMessage) error {
	for _, msg := range msgs {
		opts := []slack.MsgOption{slack.MsgOptionText(msg.Text, false)}
		if len(msg.Blocks) > 0 {
			// 空スライスで呼ぶと blocks=[] を送ることになるので渡さない（PostMessage と同じ）。
			opts = append(opts, slack.MsgOptionBlocks(msg.Blocks...))
		}
		if err := s.post(opts...); err != nil {
			return err
		}
	}
	return nil
}

// 受付を完了に差し替えることができない。新しい情報は所要時間だけで、期間・件数は
// 1 通目の context に既に出ているので、そのために 1 通増やさない（#695 決定 G-3）。
func (s *ephemeralSink) Finish(text string) {}

// リアクションを付ける先のメッセージが無い。
func (s *ephemeralSink) Close(ok bool) {}

// premortemEphemeralReceipt は slash の受付メッセージ。チャンネルに投稿できるかの
// 疎通確認も兼ねる（失敗＝bot 未参加なので、enqueue せずに response_url へ返す）。
func premortemEphemeralReceipt(sources int) string {
	if sources <= 1 {
		return "🧨 負け筋を洗い出しています。1〜2 分ほどかかります（この結果はあなただけに見えます）"
	}
	return fmt.Sprintf(
		"🧨 %d チャンネルを読んでいます。1〜2 分ほどかかります（この結果はあなただけに見えます）", sources)
}
