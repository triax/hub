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
	// 相手側の前提は 1 文。scenario と足しても section が 3 行を大きく超えないよう、
	// scenario とは独立に切り詰める（まとめて切ると文の途中で切れる）。
	premortemOpponentRuneLimit = 100
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
	// 間引きは表示の都合なので、描画用のコピーに対して行う。premortemReport は
	// 「何が採用されたか」を持つ型で、そこに描画の判断を焼き付けない（#698 決定 8）。
	risks := dedupeOpponents(report.Risks)

	blocks := make([]slack.Block, 0, premortemDigestFixedBlocks+len(risks)*premortemBlocksPerRisk)
	blocks = append(blocks,
		slack.NewHeaderBlock(slack.NewTextBlockObject(
			slack.PlainTextType, truncateRunes(premortemTitle(job, game, now), focusHeaderRuneLimit), false, false)),
		slack.NewContextBlock("premortem_meta", slack.NewTextBlockObject(
			slack.MarkdownType, premortemMeta(job, threads, now), false, false)),
		premortemTOCBlock(risks),
	)

	showUnit := premortemUnitsSpan(risks)
	total := totalRiskCount(report.Stats)
	for i, r := range risks {
		blocks = append(blocks, premortemRiskBlocks(i, r, total, showUnit)...)
	}
	blocks = append(blocks, premortemSignalBlocks(risks)...)
	return append(blocks, slack.NewDividerBlock(), slack.NewContextBlock("premortem_guide",
		slack.NewTextBlockObject(slack.MarkdownType,
			premortemGuide(job, anyRisk(risks, func(r rankedRisk) string { return r.Opponent })),
			false, false)))
}

// premortemGuide は 1 通目末尾の案内。ephemeral 配送ではスレッドが無いので
// 「このスレッドへ」が意味を成さない。共有したいときの導線に差し替える（#695）。
//
// 冒頭で射程を言い切る（#697）。メタ行にもチャンネルは出るが、あちらは数字が主で
// 読み飛ばされる。読み手が「試合そのものの予測」として受け取ると、スコープの切られた
// チャンネルから出た仮説に納得できない。どこから出た話なのかは、出力の受け取り方を
// 書くこの場所に要る。
func premortemGuide(job premortemJob, hasOpponent bool) string {
	head := premortemScopeNotice(job) + "この試合に負けるとしたら、という前提で立てた仮説です。"
	if job.Ephemeral {
		head += "これはあなただけに見えています。チームに共有するには `@" +
			BotAssistantName + " premortem` で実行してください。"
	} else {
		head += "反論・追加はこのスレッドへ。"
	}
	return head + premortemOpponentInvite(hasOpponent)
}

// premortemOpponentInvite は相手の材料が 1 件も無かったときの誘い。
//
// 「見当たりませんでした」のような欠落の報告にはしない（#698 決定 4）。欠落を詫びる書き方は
// 「相手情報が無い premortem は不完全だ」という含意を持ち、上乗せであって土台ではないという
// 大原則と矛盾する。ここは警告ではなく案内なので、context ブロックの小さい文字のまま置く。
func premortemOpponentInvite(hasOpponent bool) string {
	if hasOpponent {
		return ""
	}
	return "相手の資料をこのチャンネルに貼ると、相手を踏まえた見立てになります。"
}

// dedupeOpponents は同じ相手の前提が複数のカードで繰り返されるのを防ぐ。同じ一文が 2 枚に
// 出ると、読み手には新しい情報が増えたように見えて増えていない。順位が上のカードにだけ残す。
//
// **描画用のコピーを返し、渡された slice は書き換えない**。LLM に「重複させるな」と
// 指示しないのは、横断的な調整を頼むと数えさせることになるから（#658 の原則）。
func dedupeOpponents(risks []rankedRisk) []rankedRisk {
	out := make([]rankedRisk, len(risks))
	copy(out, risks)

	seen := map[string]bool{}
	for i := range out {
		key := normalizeForQuoteMatch(out[i].Opponent)
		if key == "" {
			continue
		}
		if seen[key] {
			out[i].premortemRisk = out[i].clearOpponent()
			continue
		}
		seen[key] = true
	}
	return out
}

// anyRisk は採用された負け筋のどれかが get の返す値を持っているか。
func anyRisk(risks []rankedRisk, get func(rankedRisk) string) bool {
	for _, r := range risks {
		if get(r) != "" {
			return true
		}
	}
	return false
}

// premortemScopeNotice は射程の申告。収集元を名指しして「ここに書かれていないことは
// 入っていない」と言う。収集元が引けないときは黙って省く（嘘を書くよりは何も言わない）。
func premortemScopeNotice(job premortemJob) string {
	scope := premortemChannelRefs(job)
	if scope == "" {
		return ""
	}
	if job.ThreadOnly {
		scope += " のこのスレッド"
	}
	return "この premortem は " + scope + " に書かれたことだけを材料にしています。"
}

// premortemChannelRefs は収集元チャンネルの参照。上限は premortemMaxSourceChannels（5）
// なので切り詰めない。どのチャンネルが入ったかが射程そのもので、省いたら申告にならない
// （#697 決定 3）。
//
// 空の ID を捨てるのは、job が Cloud Tasks の JSON を復元したものだから。組み立て時
// （newPremortemJob の uniqueStrings）は空を通さないが、payload を信用しきって `<#>` を
// 描くと読み手に壊れた参照を見せることになる。
func premortemChannelRefs(job premortemJob) string {
	refs := make([]string, 0, len(job.Sources))
	for _, id := range job.Sources {
		if id = strings.TrimSpace(id); id != "" {
			refs = append(refs, channelMention(id))
		}
	}
	return strings.Join(refs, " ")
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

// premortemMeta は見出し直下のメタ行。「どこの・いつの・何件から」を出す。
// 対象試合が引けてヘッダが試合名になると、期間はここにしか出ない。逆に引けないときは
// ヘッダと期間が重複するが、条件付きで省くと「射程の書かれない出力」が生まれるので
// 常に出す（#697 決定 1）。
//
// ThreadOnly のときは日付範囲を出さない。スレッドは時間窓で切っていないので、
// 範囲を書くと読み手に嘘をつくことになる（#697 決定 2）。
func premortemMeta(job premortemJob, threads []playThread, now time.Time) string {
	plays, _, replies := countThreadKinds(threads)
	head := premortemChannelRefs(job)
	if job.ThreadOnly {
		return joinNonEmpty([]string{head, fmt.Sprintf("このスレッドの %d 件の返信から", replies)}, " ")
	}
	span := focusRangeLabel(job.focusJobFor(job.Channel), now)
	return joinNonEmpty([]string{
		head, span, fmt.Sprintf("%d プレー / %d 件の反省と資料から", plays, replies),
	}, " · ")
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
	if body := premortemRiskBody(r); body != "" {
		head += "\n" + body
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

// premortemRiskBody は負け筋カードの本文。相手側の前提（Opponent）を Scenario の頭に
// 連結して 1 段落にする。**ラベルも専用ブロックも付けない**（#698 決定 2 = 案 E）。
//
// 専用の枠を用意すると、枠そのものが期待値を作る。相手の材料が「あるが薄い」ときに
// 見出しの下が 1 行だけ、という空席が見えてしまい、同じ情報量でも悪く読める。
// 連結なら、材料が厚ければ段落が長くなり、薄ければ 1 文増えるだけ、無ければ Scenario だけが
// 残って現行とまったく同じ見た目に戻る（#698 大原則: 相手の情報は上乗せであって土台ではない）。
func premortemRiskBody(r rankedRisk) string {
	parts := []string{}
	if r.Opponent != "" {
		parts = append(parts, endSentence(truncateRunes(r.Opponent, premortemOpponentRuneLimit)))
	}
	if r.Scenario != "" {
		parts = append(parts, truncateRunes(r.Scenario, premortemScenarioRuneLimit))
	}
	return strings.Join(parts, "")
}

// endSentence は文末に句点が無ければ足す。Opponent と Scenario を空白なしで連結するので、
// 句点が無いと 2 文が地続きに見える（日本語は語間に空白を置かないため）。
func endSentence(s string) string {
	if s == "" || strings.HasSuffix(s, "。") || strings.HasSuffix(s, "！") ||
		strings.HasSuffix(s, "？") || strings.HasSuffix(s, ".") {
		return s
	}
	return s + "。"
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
	if !anyRisk(risks, func(r rankedRisk) string { return r.Signal }) {
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
