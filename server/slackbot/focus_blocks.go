package slackbot

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slack-go/slack"
)

const (
	// Block Kit の上限。1 メッセージ 50 blocks、header の text は 150 文字、
	// section の text は 3,000 文字（Slack Block Kit の仕様）。
	focusMaxBlocksPerMessage = 50
	focusHeaderRuneLimit     = 150
	focusSectionRuneLimit    = 3000
	// 1 つの rich_text_list に詰めるプレー数。ブロック数上限の内側に収めるための安全域。
	focusMaxListItems = 25
)

// focusMessage は Slack へ 1 通として投稿する単位。
// Text は通知・検索用のフォールバック（blocks だけだと通知プレビューが空になる）。
type focusMessage struct {
	Text   string
	Blocks []slack.Block
}

// focusMessages は 1 通目（チャンネルにも出す focus の digest）と、
// スレッド内に続けるプレー別詳細を組み立てる。
func focusMessages(job focusJob, threads []playThread, now time.Time, digest focusDigest) []focusMessage {
	msgs := []focusMessage{{
		Text:   digestFallbackText(job, digest, now),
		Blocks: digestBlocks(job, threads, now, digest),
	}}
	return append(msgs, detailMessages(digest)...)
}

// digestBlocks は 1 通目。header（期間）→ context（件数）→ 番号付きの focus →
// divider → context（詳細への案内）の 5 ブロック固定。
func digestBlocks(job focusJob, threads []playThread, now time.Time, digest focusDigest) []slack.Block {
	items := make([]slack.RichTextElement, 0, len(digest.Focus))
	for _, f := range digest.Focus {
		items = append(items, slack.NewRichTextSection(focusItemElements(f)...))
	}
	return []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject(
			slack.PlainTextType, truncateRunes(digestTitle(job, now), focusHeaderRuneLimit), false, false)),
		slack.NewContextBlock("focus_meta", slack.NewTextBlockObject(
			slack.MarkdownType, digestMeta(job, threads), false, false)),
		slack.NewRichTextBlock("focus_points",
			slack.NewRichTextList(slack.RTEListOrdered, 0, items...)),
		slack.NewDividerBlock(),
		slack.NewContextBlock("focus_guide", slack.NewTextBlockObject(
			slack.MarkdownType, "▼ プレー別の詳細はこのスレッド内", false, false)),
	}
}

// focusItemElements は focus 1 点を 1 リスト項目に組む。
// 太字のタイトル ＋ ` — ` 詳細 ＋ 改行 ＋ `対象: QB, WR ／ 12 プレーで指摘`。
func focusItemElements(f focusItem) []slack.RichTextSectionElement {
	elements := []slack.RichTextSectionElement{boldElement(f.Title)}
	if detail := strings.TrimSpace(f.Detail); detail != "" {
		elements = append(elements, plainElement(" — "+detail))
	}
	if meta := focusItemMeta(f); meta != "" {
		elements = append(elements, plainElement("\n"+meta))
	}
	return elements
}

// boldElement / plainElement はリスト項目の要素。text の上限は要素ごとに掛かるので、
// どちらも同じ規則（trim して rune 境界で切り詰め）を通す。
func boldElement(s string) *slack.RichTextSectionTextElement {
	return slack.NewRichTextSectionTextElement(
		truncateRunes(strings.TrimSpace(s), focusSectionRuneLimit),
		&slack.RichTextSectionTextStyle{Bold: true})
}

func plainElement(s string) *slack.RichTextSectionTextElement {
	return slack.NewRichTextSectionTextElement(truncateRunes(s, focusSectionRuneLimit), nil)
}

func focusItemMeta(f focusItem) string {
	parts := []string{}
	if positions := joinNonEmpty(f.Positions, ", "); positions != "" {
		parts = append(parts, "対象: "+positions)
	}
	if f.Count > 0 {
		parts = append(parts, fmt.Sprintf("%d プレーで指摘", f.Count))
	}
	return strings.Join(parts, " ／ ")
}

// digestTitle は 1 通目の header。focusRangeLabel をそのまま流用しないのは、
// 日付の後ろには助詞の前に空白を置くが（`8/26〜9/7 の focus`）、
// 「このスレッド」には置かないため（`このスレッドの focus`）。
func digestTitle(job focusJob, now time.Time) string {
	if job.ThreadOnly {
		return "このスレッドの focus"
	}
	return focusRangeLabel(job, now) + " の focus"
}

func digestMeta(job focusJob, threads []playThread) string {
	plays, _, replies := countThreadKinds(threads)
	if job.ThreadOnly {
		return fmt.Sprintf("%d 件の反省から", replies)
	}
	return fmt.Sprintf("%d プレー / %d 件の反省から", plays, replies)
}

// digestFallbackText はモバイル通知プレビューと検索に出る 1 行。
func digestFallbackText(job focusJob, digest focusDigest, now time.Time) string {
	titles := make([]string, 0, len(digest.Focus))
	for i, f := range digest.Focus {
		titles = append(titles, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(f.Title)))
	}
	if len(titles) == 0 {
		return digestTitle(job, now)
	}
	return digestTitle(job, now) + ": " + strings.Join(titles, " / ")
}

// detailMessages は見出しごとのプレー別詳細を、1 メッセージ focusMaxBlocksPerMessage
// 以下に詰め直す。分割は見出し境界を優先し、1 見出しだけで上限を超える場合のみ
// プレー（リスト）の境界で割る。
func detailMessages(digest focusDigest) []focusMessage {
	msgs := []focusMessage{}
	current := focusMessage{}
	for _, section := range digest.Sections {
		for _, group := range sectionBlockGroups(section) {
			if len(current.Blocks) > 0 && len(current.Blocks)+len(group.Blocks) > focusMaxBlocksPerMessage {
				msgs = append(msgs, current)
				current = focusMessage{}
			}
			if current.Text == "" {
				current.Text = group.Text
			}
			current.Blocks = append(current.Blocks, group.Blocks...)
		}
	}
	if len(current.Blocks) > 0 {
		msgs = append(msgs, current)
	}
	return msgs
}

// sectionBlockGroups は 1 見出しぶんを「section（見出し）＋ rich_text（bullet list）」に
// 組む。プレーが多い見出しは focusMaxListItems ごとにリストを分け、それでも
// focusMaxBlocksPerMessage を超えるなら「（続き）」を付けて複数の塊に割る。
func sectionBlockGroups(section focusSection) []focusMessage {
	headline := strings.TrimSpace(section.Headline)
	if headline == "" {
		headline = "プレー別の詳細"
	}

	lists := []slack.Block{}
	for i := 0; i < len(section.Plays); i += focusMaxListItems {
		end := min(i+focusMaxListItems, len(section.Plays))
		items := make([]slack.RichTextElement, 0, end-i)
		for _, p := range section.Plays[i:end] {
			items = append(items, slack.NewRichTextSection(focusPlayElements(p)...))
		}
		lists = append(lists, slack.NewRichTextBlock("", slack.NewRichTextList(slack.RTEListBullet, 0, items...)))
	}
	if len(lists) == 0 {
		return []focusMessage{{Text: headline, Blocks: []slack.Block{sectionHeaderBlock(headline)}}}
	}

	groups := []focusMessage{}
	perGroup := focusMaxBlocksPerMessage - 1 // 見出しブロック 1 つぶんを空けておく
	for i := 0; i < len(lists); i += perGroup {
		title := headline
		if i > 0 {
			title = headline + "（続き）"
		}
		blocks := append([]slack.Block{sectionHeaderBlock(title)}, lists[i:min(i+perGroup, len(lists))]...)
		groups = append(groups, focusMessage{Text: title, Blocks: blocks})
	}
	return groups
}

func sectionHeaderBlock(headline string) slack.Block {
	return slack.NewSectionBlock(slack.NewTextBlockObject(
		slack.MarkdownType, truncateRunes("*"+headline+"*", focusSectionRuneLimit), false, false), nil, nil)
}

// focusPlayElements はプレー 1 件を 1 リスト項目に組む。
// 太字のプレー名 ＋ 改行 ＋ 反省点を `・` で連ねる。
func focusPlayElements(p focusPlay) []slack.RichTextSectionElement {
	elements := []slack.RichTextSectionElement{boldElement(p.Name)}
	if points := joinNonEmpty(p.Points, "・"); points != "" {
		elements = append(elements, plainElement("\n"+points))
	}
	return elements
}

func joinNonEmpty(values []string, sep string) string {
	kept := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			kept = append(kept, v)
		}
	}
	return strings.Join(kept, sep)
}

// truncateRunes は Block Kit の text 上限に収まるよう rune 境界で切り詰める。
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}
