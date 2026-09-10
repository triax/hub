package slackbot

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

const (
	// 1 通目の負け筋 1 点あたりの文字数。読み飛ばされない長さに抑える。
	premortemScenarioRuneLimit = 120
	premortemPreventRuneLimit  = 120
	premortemSignalRuneLimit   = 160
)

// premortemDigestFixedBlocks は 1 通目の負け筋カード以外のブロック数
// （header・context・目次 rich_text・divider・集約見出し section・集約 rich_text・divider・案内 context）。
const premortemDigestFixedBlocks = 8

// premortemBlocksPerRisk は負け筋 1 点あたりのブロック数の上限
// （divider・section・rich_text・context）。「今週やること」が 0 件なら rich_text を省くので
// 実際のブロック数はこれ以下になる。
const premortemBlocksPerRisk = 4

// 1 通目は「固定 8 ブロック + 負け筋 × 4」。上限を静かに越えないよう関係をコンパイル時に縛る
// （実測での番人は TestPremortem_DigestBlocks_MaxRisks）。
const _ = uint(focusMaxBlocksPerMessage - premortemDigestFixedBlocks - premortemMaxRisks*premortemBlocksPerRisk)

// premortemNoSignalLabel は兆候が挙がらなかった負け筋のプレースホルダ。集約リストの番号を
// カード番号と一致させるため、兆候が無い点も欠番にせず 1 行残す（#683）。
const premortemNoSignalLabel = "（試合中の兆候は挙がっていません）"

// premortemMessage は Slack へ 1 通として投稿する単位。中身は focusMessage と同一
// （Text は通知・検索用のフォールバック）なので、別の型を立てずに別名にする。
// premortem 側のコードで focusMessage と書かずに済ませるためだけの名前。
type premortemMessage = focusMessage

// premortemMessages は 1 通目（チャンネルにも出す負け筋の digest）だけを組み立てる。
// focus と違いチャートは出さない。premortem のリスクは未来の仮説で、%は過去の指摘件数。
// 並べても「この割合で崩れる」という誤読を招くだけで、読み手の判断を助けない。
// 件数の重みは目次とカードのメタ行で足りる。
func premortemMessages(job premortemJob, threads []playThread, game string, now time.Time, report premortemReport) []premortemMessage {
	return []premortemMessage{{
		Text:   premortemFallbackText(job, game, now, report),
		Blocks: premortemDigestBlocks(job, threads, game, now, report),
	}}
}

// premortemDigestBlocks は 1 通目。header（対象試合）→ context（件数）→ rich_text（目次）→
// 負け筋 1 点 = カード → divider → 「試合中に見るもの」の集約 → divider → context（案内）。
//
// 兆候（signal）をカードに入れず末尾へ集約するのは、予防策と読む人・読むタイミングが
// 違うから。予防策はコーチが今週の練習を組むときに読み、兆候はサイドラインの担当者が
// 試合中に見る。同じカードに混ぜるとどちらの用途でも余計な行を読み飛ばすことになる（#683 案D）。
func premortemDigestBlocks(job premortemJob, threads []playThread, game string, now time.Time, report premortemReport) []slack.Block {
	blocks := make([]slack.Block, 0, premortemDigestFixedBlocks+len(report.Risks)*premortemBlocksPerRisk)
	blocks = append(blocks,
		slack.NewHeaderBlock(slack.NewTextBlockObject(
			slack.PlainTextType, truncateRunes(premortemTitle(job, game, now), focusHeaderRuneLimit), false, false)),
		slack.NewContextBlock("premortem_meta", slack.NewTextBlockObject(
			slack.MarkdownType, premortemMeta(job, threads), false, false)),
		premortemTOCBlock(report.Risks),
	)

	showUnit := premortemUnitsSpan(report.Risks)
	total := totalRiskCount(report.Stats)
	for i, r := range report.Risks {
		blocks = append(blocks, premortemRiskBlocks(i, r, total, showUnit)...)
	}
	blocks = append(blocks, premortemSignalBlocks(report.Risks)...)
	return append(blocks, slack.NewDividerBlock(), slack.NewContextBlock("premortem_guide",
		slack.NewTextBlockObject(slack.MarkdownType, premortemGuide(job), false, false)))
}

// premortemGuide は 1 通目末尾の案内。ephemeral 配送ではスレッドが無いので
// 「このスレッドへ」が意味を成さない。共有したいときの導線に差し替える（#695）。
func premortemGuide(job premortemJob) string {
	const head = "この試合に負けるとしたら、という前提で立てた仮説です。"
	if job.Ephemeral {
		return head + "これはあなただけに見えています。チームに共有するには `@" +
			BotAssistantName + " premortem` で実行してください。"
	}
	return head + "反論・追加はこのスレッドへ。"
}

// premortemTitle は見出し。対象試合が引けなければ期間ラベルに落とすが、処理は止めない。
func premortemTitle(job premortemJob, game string, now time.Time) string {
	if game != "" {
		return game + " の premortem"
	}
	if job.ThreadOnly {
		return "このスレッドの premortem"
	}
	return focusRangeLabel(job.focusJobFor(job.Channel), now) + " の premortem"
}

func premortemMeta(job premortemJob, threads []playThread) string {
	plays, _, replies := countThreadKinds(threads)
	if job.ThreadOnly {
		return fmt.Sprintf("このスレッドの %d 件の返信から", replies)
	}
	return fmt.Sprintf("%d チャンネル · %d プレー / %d 件の反省と資料から",
		len(job.Sources), plays, replies)
}

// premortemTOCBlock は「何点あるのか・どれが重いのか」を先頭で読ませる目次。
func premortemTOCBlock(risks []rankedRisk) slack.Block {
	items := make([]slack.RichTextElement, 0, len(risks))
	for _, r := range risks {
		elements := []slack.RichTextSectionElement{boldElement(r.Label)}
		if meta := premortemTOCMeta(r); meta != "" {
			elements = append(elements, plainElement("　"+meta))
		}
		items = append(items, slack.NewRichTextSection(elements...))
	}
	return slack.NewRichTextBlock("premortem_toc", slack.NewRichTextList(slack.RTEListOrdered, 0, items...))
}

func premortemTOCMeta(r rankedRisk) string {
	parts := []string{}
	if positions := joinNonEmpty(r.Positions, ", "); positions != "" {
		parts = append(parts, positions)
	}
	if r.Count > 0 {
		parts = append(parts, fmt.Sprintf("%d プレー", r.Count))
	}
	return strings.Join(parts, " · ")
}

// premortemRiskBlocks は負け筋 1 点のカード。兆候はここには出さない（末尾に集約する）。
func premortemRiskBlocks(i int, r rankedRisk, total int, showUnit bool) []slack.Block {
	// 見出しと概要は別々に切り詰める（まとめて切ると太字の `*` を落として markdown が壊れる）。
	head := fmt.Sprintf("*%d. %s*", i+1, truncateRunes(r.Title, focusHeaderRuneLimit))
	if r.Scenario != "" {
		head += "\n" + truncateRunes(r.Scenario, premortemScenarioRuneLimit)
	}
	blocks := []slack.Block{
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, head, false, false), nil, nil),
	}
	if items := premortemPreventItems(r); len(items) > 0 {
		blocks = append(blocks, slack.NewRichTextBlock(fmt.Sprintf("premortem_risk_%d", i+1),
			slack.NewRichTextList(slack.RTEListBullet, 0, items...)))
	}
	if meta := premortemRiskMeta(r, total, showUnit); meta != "" {
		blocks = append(blocks, slack.NewContextBlock(fmt.Sprintf("premortem_risk_meta_%d", i+1),
			slack.NewTextBlockObject(slack.MarkdownType, meta, false, false)))
	}
	return blocks
}

// premortemPreventItems は「今週やること」。空のリストは invalid_blocks で弾かれるので、
// 0 件なら呼び出し側が rich_text ごと省く。
func premortemPreventItems(r rankedRisk) []slack.RichTextElement {
	items := make([]slack.RichTextElement, 0, len(r.Prevent))
	for _, a := range r.Prevent {
		items = append(items, slack.NewRichTextSection(
			plainElement("🛡 "+truncateRunes(a, premortemPreventRuneLimit))))
	}
	return items
}

// premortemRiskMeta はカード下のメタ行。unit はチャンネルがスコープを規定しているのが
// 基本なので、採用リスクが 2 ユニット以上にまたがるときだけ出す（#683）。
func premortemRiskMeta(r rankedRisk, total int, showUnit bool) string {
	parts := []string{}
	if r.Phase != "" {
		parts = append(parts, r.Phase)
	}
	if showUnit && r.Unit != "" {
		parts = append(parts, r.Unit)
	}
	if positions := joinNonEmpty(r.Positions, ", "); positions != "" {
		parts = append(parts, positions)
	}
	if r.Count > 0 {
		count := fmt.Sprintf("%d プレー", r.Count)
		if total > 0 {
			count += fmt.Sprintf("（%d%%）", int(math.Round(float64(r.Count)/float64(total)*100)))
		}
		parts = append(parts, count)
	}
	head := strings.Join(parts, " · ")
	if r.Quote == "" {
		return head
	}
	if head == "" {
		return "「" + r.Quote + "」"
	}
	return head + "　·　「" + r.Quote + "」"
}

// premortemSignalBlocks は「⚠️ 試合中に見るもの」。試合当日はここだけ見れば済むように、
// 全部の兆候を 1 ブロックに集める。番号はカードの番号と一致する（兆候の無い点も欠番にしない）。
// 全点に兆候が無いときは、見出しごと出さない（空の rich_text_list は invalid_blocks）。
func premortemSignalBlocks(risks []rankedRisk) []slack.Block {
	if !hasAnySignal(risks) {
		return nil
	}
	items := make([]slack.RichTextElement, 0, len(risks))
	for _, r := range risks {
		elements := []slack.RichTextSectionElement{}
		if r.Phase != "" {
			elements = append(elements, plainElement("["+r.Phase+"] "))
		}
		if r.Signal == "" {
			elements = append(elements, plainElement(premortemNoSignalLabel))
		} else {
			elements = append(elements, plainElement(truncateRunes(r.Signal, premortemSignalRuneLimit)))
		}
		items = append(items, slack.NewRichTextSection(elements...))
	}
	return []slack.Block{
		slack.NewDividerBlock(),
		slack.NewSectionBlock(slack.NewTextBlockObject(
			slack.MarkdownType, "*⚠️ 試合中に見るもの*", false, false), nil, nil),
		slack.NewRichTextBlock("premortem_signals",
			slack.NewRichTextList(slack.RTEListOrdered, 0, items...)),
	}
}

func hasAnySignal(risks []rankedRisk) bool {
	for _, r := range risks {
		if r.Signal != "" {
			return true
		}
	}
	return false
}

func totalRiskCount(stats premortemStats) int {
	total := 0
	for _, r := range stats.Risks {
		total += r.Count
	}
	return total
}

// premortemFallbackText は通知・検索に出る 1 行（blocks だけだと空になる）。
func premortemFallbackText(job premortemJob, game string, now time.Time, report premortemReport) string {
	labels := make([]string, 0, len(report.Risks))
	for i, r := range report.Risks {
		labels = append(labels, fmt.Sprintf("%d. %s", i+1, r.Label))
	}
	title := premortemTitle(job, game, now)
	if len(labels) == 0 {
		return title
	}
	return title + ": " + strings.Join(labels, " / ")
}
