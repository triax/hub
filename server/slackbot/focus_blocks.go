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
	// 1 通目の focus 1 点あたりの文字数。読み飛ばされない長さに抑える
	// （Block Kit の上限より手前で切る意図的な制約）。
	focusSummaryRuneLimit = 300
	focusActionRuneLimit  = 120
)

// focusDigestFixedBlocks は 1 通目の focus 以外のブロック数（header・context・
// divider・context）。digestBlocks の容量計算と下の上限チェックが同じ数を見るように、
// マジックナンバーをここ 1 箇所に置く。
const focusDigestFixedBlocks = 4

// 1 通目は「固定 4 ブロック + focus 件数ぶんの rich_text」。focus の上限を増やしたときに
// ブロック数上限を静かに越えないよう、関係をコンパイル時に縛る
// （focusMaxThemes を増やしたらここで落ちる。実測での番人は
// TestFocus_DigestBlocks_MaxThemes）。
const _ = uint(focusMaxBlocksPerMessage - focusDigestFixedBlocks - focusMaxThemes)

// focusMessage は Slack へ 1 通として投稿する単位。
// Text は通知・検索用のフォールバック（blocks だけだと通知プレビューが空になる）。
type focusMessage struct {
	Text   string
	Blocks []slack.Block
}

// focusMessages は 1 通目（チャンネルにも出す focus の digest）と、
// スレッド内に続けるチャート 1 通を組み立てる。プレー別の一覧は
// `focus 12d full` のときだけチャートの後に続ける（既定では出さない）。
func focusMessages(job focusJob, threads []playThread, now time.Time, report focusReport) []focusMessage {
	msgs := []focusMessage{{
		Text:   digestFallbackText(job, report, now),
		Blocks: digestBlocks(job, threads, now, report),
	}}
	if chart, ok := focusChartMessage(report); ok {
		msgs = append(msgs, chart)
	}
	if job.Full {
		msgs = append(msgs, detailMessages(report)...)
	}
	return msgs
}

// digestBlocks は 1 通目。header（期間）→ context（件数）→ focus 1 点 = 1 ブロック →
// divider → context（詳細への案内）。番号は title に含めるので ordered list は使わない
// （rich_text は入れ子のリストを持てず、段下げは list の indent で表すため）。
func digestBlocks(job focusJob, threads []playThread, now time.Time, report focusReport) []slack.Block {
	blocks := make([]slack.Block, 0, focusDigestFixedBlocks+len(report.Focus))
	blocks = append(blocks,
		slack.NewHeaderBlock(slack.NewTextBlockObject(
			slack.PlainTextType, truncateRunes(digestTitle(job, now), focusHeaderRuneLimit), false, false)),
		slack.NewContextBlock("focus_meta", slack.NewTextBlockObject(
			slack.MarkdownType, digestMeta(job, threads), false, false)),
	)
	for i, f := range report.Focus {
		blocks = append(blocks, focusItemBlock(i, f))
	}
	return append(blocks,
		slack.NewDividerBlock(),
		slack.NewContextBlock("focus_guide", slack.NewTextBlockObject(
			slack.MarkdownType, digestGuide(job, report), false, false)),
	)
}

// digestGuide はスレッドに何が続くかの案内。既定はチャート 1 通で、
// プレー別の一覧は `full` を付けたときだけ続く。
func digestGuide(job focusJob, report focusReport) string {
	switch {
	case job.Full:
		return "▼ 内訳のチャートとプレー別の詳細はこのスレッド内"
	case !report.Stats.empty():
		return "▼ 内訳のチャートはこのスレッド内（プレー別の一覧は `full` を付けて再実行）"
	default:
		return "▼ プレー別の一覧は `full` を付けて再実行"
	}
}

// focusItemBlock は focus 1 点を 1 つの rich_text ブロックに組む。要素の並びは
// 「太字の `N. タイトル` ＋ 改行 ＋ 概要」→「太字の やる ＋ 段下げ bullet」→
// 「太字の やめる ＋ 段下げ bullet」→「対象・件数・引用の補足行」。
// 「やる」「やめる」は該当が無ければラベルごと省く（空の rich_text_list は
// Slack に invalid_blocks で弾かれるため、省略は見た目の都合だけではない）。
func focusItemBlock(i int, f rankedTheme) slack.Block {
	title := boldElement(fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(f.Title)))
	head := []slack.RichTextSectionElement{title}
	if summary := strings.TrimSpace(f.Summary); summary != "" {
		head = append(head, plainElement("\n"+truncateRunes(summary, focusSummaryRuneLimit)))
	}

	elements := []slack.RichTextElement{slack.NewRichTextSection(head...)}
	elements = append(elements, focusActionElements("やる", f.Do)...)
	elements = append(elements, focusActionElements("やめる", f.Dont)...)
	if meta := focusItemMeta(f); meta != "" {
		elements = append(elements, slack.NewRichTextSection(plainElement(meta)))
	}
	return slack.NewRichTextBlock(fmt.Sprintf("focus_item_%d", i+1), elements...)
}

// focusActionElements は「やる」「やめる」の 1 段（太字のラベル ＋ 段下げした bullet）。
// 該当が無ければ空を返し、呼び出し側でラベルごと消える。
func focusActionElements(label string, actions []string) []slack.RichTextElement {
	if len(actions) == 0 {
		return nil
	}
	items := make([]slack.RichTextElement, 0, len(actions))
	for _, a := range actions {
		items = append(items, slack.NewRichTextSection(plainElement(truncateRunes(a, focusActionRuneLimit))))
	}
	return []slack.RichTextElement{
		slack.NewRichTextSection(boldElement(label)),
		slack.NewRichTextList(slack.RTEListBullet, 1, items...),
	}
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

func focusItemMeta(f rankedTheme) string {
	parts := []string{}
	if positions := joinNonEmpty(f.Positions, ", "); positions != "" {
		parts = append(parts, "対象: "+positions)
	}
	if f.Count > 0 {
		parts = append(parts, fmt.Sprintf("%d プレー", f.Count))
	}
	if quote := strings.TrimSpace(f.Quote); quote != "" {
		parts = append(parts, "「"+quote+"」")
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
func digestFallbackText(job focusJob, report focusReport, now time.Time) string {
	titles := make([]string, 0, len(report.Focus))
	for i, f := range report.Focus {
		titles = append(titles, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(f.Title)))
	}
	if len(titles) == 0 {
		return digestTitle(job, now)
	}
	return digestTitle(job, now) + ": " + strings.Join(titles, " / ")
}

// focusSection は見出し（ドリルやシリーズの区切り）とその配下のプレー。
// LLM は plays をフラットに返すので、描画の直前にここへ畳み直す。
type focusSection struct {
	Headline string
	Plays    []focusPlay
}

// groupPlaysByHeadline は plays を見出しの初出順にまとめる。見出しの表示名は
// focusHeadlineLabel が決める（チャートの x 軸と同じ区切りになる）。
func groupPlaysByHeadline(plays []focusPlay) []focusSection {
	sections := []focusSection{}
	index := map[string]int{}
	for _, play := range plays {
		headline := focusHeadlineLabel(play.Headline)
		at, seen := index[headline]
		if !seen {
			at = len(sections)
			index[headline] = at
			sections = append(sections, focusSection{Headline: headline})
		}
		sections[at].Plays = append(sections[at].Plays, play)
	}
	return sections
}

// detailMessages は見出しごとのプレー別詳細を、1 メッセージ focusMaxBlocksPerMessage
// 以下に詰め直す。分割は見出し境界を優先し、1 見出しだけで上限を超える場合のみ
// プレー（リスト）の境界で割る。
func detailMessages(report focusReport) []focusMessage {
	msgs := []focusMessage{}
	current := focusMessage{}
	for _, section := range groupPlaysByHeadline(report.Plays) {
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
	headline := section.Headline // groupPlaysByHeadline が空欄を寄せ済み

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
// 太字のプレー名 ＋ 改行 ＋ そのプレーで指摘された事実。
func focusPlayElements(p focusPlay) []slack.RichTextSectionElement {
	elements := []slack.RichTextSectionElement{boldElement(p.Name)}
	if issue := strings.TrimSpace(p.Issue); issue != "" {
		elements = append(elements, plainElement("\n"+issue))
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
