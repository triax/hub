package slackbot

import (
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

func premortemChartOf(t *testing.T, report premortemReport) *slack.DataVisualizationBlock {
	t.Helper()
	msg, ok := premortemChartMessage(report)
	if !ok {
		t.Fatal("チャートが組まれていない")
	}
	return msg.Blocks[0].(*slack.DataVisualizationBlock)
}

// AC-21: pie は「根拠になった指摘の内訳」、bar はポジション別。
func TestPremortemChartMessage(t *testing.T) {
	report := rankRisks(premortemRiskFixture(), false)
	msg, ok := premortemChartMessage(report)
	if !ok {
		t.Fatal("チャートが組まれていない")
	}
	if len(msg.Blocks) != 2 {
		t.Fatalf("blocks = %d, want pie + bar の 2 つ", len(msg.Blocks))
	}

	pieBlock := msg.Blocks[0].(*slack.DataVisualizationBlock)
	if pieBlock.Title != premortemPieTitle {
		t.Fatalf("pie のタイトル = %q, want %q（予測ではなく実測だと分かる書き方）",
			pieBlock.Title, premortemPieTitle)
	}
	if strings.Contains(premortemPieTitle, "崩れどころ") {
		t.Fatal("pie のタイトルが予測の割合に読める")
	}
	pie, ok := pieBlock.Chart.(*slack.DataVisualizationPieChart)
	if !ok {
		t.Fatalf("1 個目が pie でない: %T", pieBlock.Chart)
	}
	// 採用 3 点 + 採用外（extra, 1 件）を畳んだ「その他」
	if len(pie.Segments) != premortemMaxRisks+1 {
		t.Fatalf("segments = %d, want %d（採用 %d + その他）",
			len(pie.Segments), premortemMaxRisks+1, premortemMaxRisks)
	}
	labels := make([]string, 0, len(pie.Segments))
	for _, s := range pie.Segments {
		labels = append(labels, s.Label)
	}
	want := "3rd&long の被サック,相手のラン想定違い,修正コールの遅れ,その他"
	if got := strings.Join(labels, ","); got != want {
		t.Fatalf("凡例 = %q, want %q", got, want)
	}
	for _, l := range labels {
		if strings.HasSuffix(l, "…") {
			t.Fatalf("凡例が切り詰められている: %q", l)
		}
	}

	barBlock := msg.Blocks[1].(*slack.DataVisualizationBlock)
	if barBlock.Title != premortemBarTitle {
		t.Fatalf("bar のタイトル = %q", barBlock.Title)
	}
	if _, ok := barBlock.Chart.(*slack.DataVisualizationBarChart); !ok {
		t.Fatalf("2 個目が bar でない: %T", barBlock.Chart)
	}
	if strings.TrimSpace(msg.Text) == "" {
		t.Fatal("チャートの fallback text が空")
	}
	if !strings.HasPrefix(msg.Text, premortemPieTitle+": ") {
		t.Fatalf("fallback text = %q", msg.Text)
	}
}

// AC-21: ポジションが 1 種類しか無い入力では bar を出さない（比較にならないため）。
func TestPremortemChartMessage_SinglePosition(t *testing.T) {
	d := premortemRiskFixture()
	for i := range d.Plays {
		d.Plays[i].Positions = []string{"OL"}
	}
	msg, ok := premortemChartMessage(rankRisks(d, false))
	if !ok {
		t.Fatal("チャートが組まれていない")
	}
	if len(msg.Blocks) != 1 {
		t.Fatalf("blocks = %d, want pie だけ", len(msg.Blocks))
	}
	if premortemChartOf(t, rankRisks(d, false)).Title != premortemPieTitle {
		t.Fatal("残った 1 枚が pie でない")
	}
}

// 集計が空（平文フォールバック経路）ならチャートを出さない。
func TestPremortemChartMessage_Empty(t *testing.T) {
	if _, ok := premortemChartMessage(premortemReport{}); ok {
		t.Fatal("集計が空なのにチャートを組んでいる")
	}
}

// ポジション上限の「その他」畳み込みは positionSeriesFrom（focus と共有）の責務で、
// TestFocusChartMessage_ManyPositions が番人になっているのでここでは重ねて検証しない。
