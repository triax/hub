package slackbot

import (
	"fmt"
	"math"
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
	// （Block Kit の上限より手前で切る意図的な制約）。カード化で 1 テーマの縦が
	// 伸びたぶん、概要が 1〜2 文に収まる長さまで詰めた（#681）。
	focusSummaryRuneLimit = 120
	focusActionRuneLimit  = 120
)

// focusDigestFixedBlocks は 1 通目の focus 以外のブロック数（header・context・
// 目次の rich_text・divider・context）。digestBlocks の容量計算と下の上限チェックが
// 同じ数を見るように、マジックナンバーをここ 1 箇所に置く。
const focusDigestFixedBlocks = 5

// focusBlocksPerTheme は focus 1 点あたりのブロック数の上限（divider・section・
// rich_text・context）。「やる」「やめる」が両方 0 件のテーマは rich_text を省くので、
// 実際のブロック数はこれ以下になる。
const focusBlocksPerTheme = 4

// 1 通目は「固定 5 ブロック + focus 件数 × focusBlocksPerTheme」。focus の上限や
// カードの構成を増やしたときにブロック数上限を静かに越えないよう、関係をコンパイル時に
// 縛る（実測での番人は TestFocus_DigestBlocks_MaxThemes）。
const _ = uint(focusMaxBlocksPerMessage - focusDigestFixedBlocks - focusMaxThemes*focusBlocksPerTheme)

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

// digestBlocks は 1 通目。header（期間）→ context（件数）→ rich_text（目次）→
// focus 1 点 = カード（focusItemBlocks）→ divider → context（詳細への案内）。
// 目次を先に置くことで「何点あるのか・どれが重いのか」が最初に読める（#681）。
func digestBlocks(job focusJob, threads []playThread, now time.Time, report focusReport) []slack.Block {
	blocks := make([]slack.Block, 0, focusDigestFixedBlocks+len(report.Focus)*focusBlocksPerTheme)
	blocks = append(blocks,
		slack.NewHeaderBlock(slack.NewTextBlockObject(
			slack.PlainTextType, truncateRunes(digestTitle(job, now), focusHeaderRuneLimit), false, false)),
		slack.NewContextBlock("focus_meta", slack.NewTextBlockObject(
			slack.MarkdownType, digestMeta(job, threads), false, false)),
		focusTOCBlock(report.Focus),
	)
	total := totalThemeCount(report.Stats)
	for i, f := range report.Focus {
		blocks = append(blocks, focusItemBlocks(i, f, total)...)
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

// focusTOCBlock は 1 通目の冒頭に置く目次。番号は ordered list に振らせ、各カードの
// 見出し（`N. title`）と一致させる。行に出すのは短い label で、長い title はカード側の
// 見出しに残す（役割が違うので両方持つ）。
func focusTOCBlock(focus []rankedTheme) slack.Block {
	items := make([]slack.RichTextElement, 0, len(focus))
	for _, f := range focus {
		elements := []slack.RichTextSectionElement{boldElement(f.Label)}
		if meta := focusThemeMeta(f); meta != "" {
			elements = append(elements, plainElement("　"+meta))
		}
		items = append(items, slack.NewRichTextSection(elements...))
	}
	return slack.NewRichTextBlock("focus_toc", slack.NewRichTextList(slack.RTEListOrdered, 0, items...))
}

// focusThemeMeta は目次 1 行とカード末尾に共通の補足（`WR, H · 27 プレー`）。
// ポジションが取れていないテーマでは中黒ごと省く（「不明」はチャートの穴埋め
// ラベルで、テーマ側の欠落とは別の概念なので目次には持ち込まない）。
func focusThemeMeta(f rankedTheme) string {
	parts := []string{}
	if positions := joinNonEmpty(f.Positions, ", "); positions != "" {
		parts = append(parts, positions)
	}
	if f.Count > 0 {
		parts = append(parts, fmt.Sprintf("%d プレー", f.Count))
	}
	return strings.Join(parts, " · ")
}

// focusItemBlocks は focus 1 点を 1 枚のカードに組む。divider（前のテーマとの区切り）
// → section（`N. title` ＋ 概要）→ rich_text（✅ / 🚫 の bullet）→ context（ポジション・
// 件数・割合・引用）。「やる」「やめる」が両方 0 件なら rich_text を省く（空の
// rich_text_list は Slack に invalid_blocks で弾かれるため、省略は見た目の都合だけではない）。
func focusItemBlocks(i int, f rankedTheme, total int) []slack.Block {
	// f.Title / f.Summary / f.Quote は normalizeTheme（focus_rank.go）で trim 済み。
	// 見出しと概要は別々に切り詰める（まとめて切ると太字の `*` を落として
	// markdown が壊れる）。
	head := fmt.Sprintf("*%d. %s*", i+1, truncateRunes(f.Title, focusHeaderRuneLimit))
	if f.Summary != "" {
		head += "\n" + truncateRunes(f.Summary, focusSummaryRuneLimit)
	}
	blocks := []slack.Block{
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, head, false, false), nil, nil),
	}
	if actions := focusActionItems(f); len(actions) > 0 {
		blocks = append(blocks, slack.NewRichTextBlock(fmt.Sprintf("focus_item_%d", i+1),
			slack.NewRichTextList(slack.RTEListBullet, 0, actions...)))
	}
	if meta := focusItemMeta(f, total); meta != "" {
		blocks = append(blocks, slack.NewContextBlock(fmt.Sprintf("focus_item_meta_%d", i+1),
			slack.NewTextBlockObject(slack.MarkdownType, meta, false, false)))
	}
	return blocks
}

// focusActionItems は「やる」「やめる」を 1 つの bullet リストに畳む。ラベル行を持たせず
// ✅ / 🚫 を各行の先頭に置くことで、片方が 0 件でも空のリストを作らずに済む
// （かつカード 1 枚の縦が短くなる）。
func focusActionItems(f rankedTheme) []slack.RichTextElement {
	items := make([]slack.RichTextElement, 0, len(f.Do)+len(f.Dont))
	for _, group := range []struct {
		prefix  string
		actions []string
	}{{"✅ ", f.Do}, {"🚫 ", f.Dont}} {
		for _, a := range group.actions {
			items = append(items, slack.NewRichTextSection(
				plainElement(group.prefix+truncateRunes(a, focusActionRuneLimit))))
		}
	}
	return items
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

// focusItemMeta はカードの末尾に置く補足（`WR, H · 27 プレー（34%）　·　「引用」`）。
// 割合の分母は集計（focusStats.Themes）の件数合計で、pie の 1 切れと同じものを指す。
func focusItemMeta(f rankedTheme, total int) string {
	head := focusThemeMeta(f) // 末尾が「N プレー」なので、割合はそのまま後ろに付く
	if f.Count > 0 && total > 0 {
		head += fmt.Sprintf("（%d%%）", int(math.Round(float64(f.Count)/float64(total)*100)))
	}
	if f.Quote == "" {
		return head
	}
	if head == "" {
		return "「" + f.Quote + "」"
	}
	return head + "　·　「" + f.Quote + "」"
}

// totalThemeCount は集計に残った全テーマの件数合計。focus に採らなかったテーマも含む
// （カードの割合と pie の 1 切れが同じ分母を見るようにするため）。
func totalThemeCount(stats focusStats) int {
	total := 0
	for _, t := range stats.Themes {
		total += t.Count
	}
	return total
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
	// 通知プレビューは短いほうが効くので、長い title ではなく label を並べる。
	titles := make([]string, 0, len(report.Focus))
	for i, f := range report.Focus {
		titles = append(titles, fmt.Sprintf("%d. %s", i+1, f.Label))
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
