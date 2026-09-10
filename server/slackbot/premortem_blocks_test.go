package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
)

// blocksJSON は blocks を検査用の 1 本の JSON 文字列にする。encoding/json の既定は
// `&` を \u0026 にエスケープしてしまい、"3rd&long" のような実データが素直に照合できない。
func blocksJSON(t *testing.T, blocks []slack.Block) string {
	t.Helper()
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(marshalBlocks(t, blocks)); err != nil {
		t.Fatalf("encode blocks: %v", err)
	}
	return buf.String()
}

func premortemTestJob() premortemJob {
	return premortemJob{Channel: "C1", Sources: []string{"C1"}, MentionTS: testMentionTS, Oldest: 0}
}

func premortemSampleReport(t *testing.T) premortemReport {
	t.Helper()
	return rankRisks(premortemRiskFixture(), false)
}

// AC-11 / AC-13 / AC-31 / AC-32: 案D の並びと中身。
func TestPremortem_DigestBlocks_PlanD(t *testing.T) {
	report := premortemSampleReport(t)
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "9/21(日) vs シルバースター", now, report)

	want := "header,context,rich_text," +
		strings.Repeat("divider,section,rich_text,context,", premortemMaxRisks) +
		"divider,section,rich_text,divider,context"
	if got := strings.Join(blockTypes(blocks), ","); got != want {
		t.Fatalf("block 構成が案D と違う:\n got=%s\nwant=%s", got, want)
	}
	if len(blocks) != premortemDigestFixedBlocks+premortemMaxRisks*premortemBlocksPerRisk {
		t.Fatalf("block 数 = %d, want %d", len(blocks),
			premortemDigestFixedBlocks+premortemMaxRisks*premortemBlocksPerRisk)
	}

	raw := marshalBlocks(t, blocks)
	body := blocksJSON(t, blocks)

	// header は対象試合
	if !strings.Contains(body, "9/21(日) vs シルバースター の premortem") {
		t.Fatalf("header に対象試合が入っていない: %s", body[:200])
	}
	// AC-13: 集約セクションがあり、兆候はカード内に出ない（重複しない）
	if !strings.Contains(body, "⚠️ 試合中に見るもの") {
		t.Fatal("「試合中に見るもの」の集約セクションが無い")
	}
	signal := report.Risks[0].Signal
	if n := strings.Count(body, signal); n != 1 {
		t.Fatalf("兆候の出現回数 = %d, want 1（カードに出さず集約にだけ出す）", n)
	}
	// 集約の番号はカードの番号と一致する = 採用点数ぶんの項目が並ぶ
	signals := raw[len(raw)-3]
	items := signals["elements"].([]any)[0].(map[string]any)["elements"].([]any)
	if len(items) != len(report.Risks) {
		t.Fatalf("集約リストの項目数 = %d, want %d（カード番号と一致させる）", len(items), len(report.Risks))
	}
	// AC-31: kind の値はどこにも出ない
	for _, kind := range premortemKinds {
		if strings.Contains(body, kind) {
			t.Fatalf("kind %q が出力に漏れている", kind)
		}
	}
	// AC-32: phase はメタ行に出る。unit は 2 ユニットにまたがるので出る（AC-33 の片側）
	if !strings.Contains(body, "3rd&long · OF · OL, QB · 3 プレー（38%）") {
		t.Fatalf("カードのメタ行が期待どおりでない: %s", body)
	}
}

// AC-33: 採用リスクの unit が 1 種類なら、unit をどのブロックにも出さない。
func TestPremortem_DigestBlocks_SingleUnitHidesUnit(t *testing.T) {
	d := premortemRiskFixture()
	for i := range d.Risks {
		d.Risks[i].Unit = "OF" // #offence のようにスコープの定まったチャンネル
	}
	report := rankRisks(d, false)
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "", time.Now(), report)

	body := blocksJSON(t, blocks)
	if strings.Contains(body, "· OF ·") || strings.Contains(body, "OF · OL") {
		t.Fatalf("1 ユニットしか無いのに unit が出ている: %s", body)
	}
	// phase は残る
	if !strings.Contains(body, "3rd&long · OL, QB") {
		t.Fatalf("unit を落としたら phase / ポジションまで消えた: %s", body)
	}
}

// AC-12: prevent が空のリスクではカードの rich_text を出さない。
func TestPremortem_DigestBlocks_NoPrevent(t *testing.T) {
	d := premortemRiskFixture()
	for i := range d.Risks {
		d.Risks[i].Prevent = nil
	}
	report := rankRisks(d, false)
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "", time.Now(), report)

	want := "header,context,rich_text," +
		strings.Repeat("divider,section,context,", premortemMaxRisks) +
		"divider,section,rich_text,divider,context"
	if got := strings.Join(blockTypes(blocks), ","); got != want {
		t.Fatalf("prevent 空のとき rich_text を省いていない:\n got=%s\nwant=%s", got, want)
	}
}

// AC-14: 全リスクの signal が空 → 集約セクションごと出さず、それ以外の並びは崩れない。
func TestPremortem_DigestBlocks_NoSignals(t *testing.T) {
	d := premortemRiskFixture()
	for i := range d.Risks {
		d.Risks[i].Signal = ""
	}
	report := rankRisks(d, false)
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "", time.Now(), report)

	want := "header,context,rich_text," +
		strings.Repeat("divider,section,rich_text,context,", premortemMaxRisks) +
		"divider,context"
	if got := strings.Join(blockTypes(blocks), ","); got != want {
		t.Fatalf("signal が全て空のとき集約を省いていない:\n got=%s\nwant=%s", got, want)
	}
}

// signal が一部だけ空でも、集約リストの番号はカード番号とずれない（欠番にしない）。
func TestPremortem_DigestBlocks_PartialSignals(t *testing.T) {
	d := premortemRiskFixture()
	d.Risks[1].Signal = "" // 2 点目だけ兆候なし
	report := rankRisks(d, false)
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "", time.Now(), report)

	raw := marshalBlocks(t, blocks)
	signals := raw[len(raw)-3]
	items := signals["elements"].([]any)[0].(map[string]any)["elements"].([]any)
	if len(items) != len(report.Risks) {
		t.Fatalf("集約リストの項目数 = %d, want %d", len(items), len(report.Risks))
	}
	item, _ := json.Marshal(items[1])
	if !strings.Contains(string(item), premortemNoSignalLabel) {
		t.Fatalf("兆候の無い点が欠番になっている: %s", item)
	}
}

// ブロック数上限の実測での番人（コンパイル時 assert の裏取り）。
func TestPremortem_DigestBlocks_MaxRisks(t *testing.T) {
	report := premortemSampleReport(t)
	if len(report.Risks) != premortemMaxRisks {
		t.Fatalf("採用 = %d, want %d", len(report.Risks), premortemMaxRisks)
	}
	blocks := premortemDigestBlocks(premortemTestJob(), playThreads(8), "9/21(日) vs A", time.Now(), report)
	if len(blocks) > focusMaxBlocksPerMessage {
		t.Fatalf("block 数 = %d, Slack の上限 %d を超えている", len(blocks), focusMaxBlocksPerMessage)
	}
}

// AC-15 / AC-16: 対象試合が引ければ見出しに入り、引けなければ期間ラベルに落ちる。
func TestPremortemTitle(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	job := premortemTestJob()
	job.Oldest = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC).Unix()

	if got := premortemTitle(job, "9/21(日) vs シルバースター", now); got != "9/21(日) vs シルバースター の premortem" {
		t.Fatalf("試合ありの見出し = %q", got)
	}
	got := premortemTitle(job, "", now)
	if !strings.HasSuffix(got, " の premortem") || strings.Contains(got, "vs") {
		t.Fatalf("試合なしの見出し = %q, want 期間ラベル", got)
	}
	if !strings.Contains(got, "9/10") {
		t.Fatalf("期間ラベルに終端の日付が入っていない: %q", got)
	}

	threadOnly := premortemJob{Channel: "C1", Sources: []string{"C1"}, MentionTS: testMentionTS, ThreadOnly: true}
	if got := premortemTitle(threadOnly, "", now); got != "このスレッドの premortem" {
		t.Fatalf("スレッド単体の見出し = %q", got)
	}
}

// AC-22: fallback text（通知プレビュー）が空にならない。
func TestPremortem_FallbackText(t *testing.T) {
	report := premortemSampleReport(t)
	now := time.Now()
	msgs := premortemMessages(premortemTestJob(), playThreads(8), "9/21(日) vs A", now, report)
	if len(msgs) < 2 {
		t.Fatalf("messages = %d, want 1 通目 + チャート", len(msgs))
	}
	for i, m := range msgs {
		if strings.TrimSpace(m.Text) == "" {
			t.Fatalf("messages[%d].Text が空（blocks だけだと通知が空になる）", i)
		}
	}
	want := "9/21(日) vs A の premortem: 1. 3rd&long の被サック / 2. 相手のラン想定違い / 3. 修正コールの遅れ"
	if msgs[0].Text != want {
		t.Fatalf("fallback text =\n %q\nwant\n %q", msgs[0].Text, want)
	}
}

// phase が空でも区切り記号だけが残らない（AC-32 の裏側）。
func TestPremortemRiskMeta_EmptyFields(t *testing.T) {
	r := rankedRisk{premortemRisk: premortemRisk{Phase: "", Unit: "OF"}, Count: 0}
	if got := premortemRiskMeta(r, 0, false); got != "" {
		t.Fatalf("すべて空なのにメタ行が出た: %q", got)
	}
	r.Quote = "引用だけ"
	if got := premortemRiskMeta(r, 0, false); got != "「引用だけ」" {
		t.Fatalf("引用だけのメタ行 = %q", got)
	}
	r = rankedRisk{premortemRisk: premortemRisk{Phase: "3rd&long"}, Count: 2}
	if got := premortemRiskMeta(r, 8, false); got != "3rd&long · 2 プレー（25%）" {
		t.Fatalf("メタ行 = %q", got)
	}
}

// 目次は「何点あるか・どれが重いか」を先頭で読ませる。
func TestPremortem_TOCBlock(t *testing.T) {
	report := premortemSampleReport(t)
	body := blocksJSON(t, []slack.Block{premortemTOCBlock(report.Risks)})
	for i, want := range []string{
		"3rd&long の被サック", "OL, QB · 3 プレー",
		"相手のラン想定違い", "DL, LB · 2 プレー",
		"修正コールの遅れ", "DB, LB · 2 プレー",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("目次に %q（%d 番目の期待値）が無い: %s", want, i, body)
		}
	}
	if strings.Contains(body, fmt.Sprint(premortemKindExecution)) {
		t.Fatal("目次に kind が漏れている")
	}
}
