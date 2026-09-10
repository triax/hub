package slackbot

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/slack-go/slack"
)

// chartFixture は テーマ 6・見出し 4・ポジション 5 の集計を作る。
func chartFixture() focusDigest {
	positions := []string{"QB", "WR", "OL", "TE", "RB"}
	d := focusDigest{}
	for i := 0; i < 6; i++ {
		d.Themes = append(d.Themes, focusTheme{Key: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("テーマ%d", i)})
	}
	// テーマ t0 が最多になるよう、若い番号ほど多くのプレーに紐づける。
	for i := 0; i < 18; i++ {
		play := focusPlay{
			Headline:  fmt.Sprintf("見出し%d", i%4),
			Name:      fmt.Sprintf("プレー%02d", i),
			Positions: []string{positions[i%5]},
		}
		for j := 0; j < 6; j++ {
			if i%(j+2) == 0 {
				play.ThemeKeys = append(play.ThemeKeys, fmt.Sprintf("t%d", j))
			}
		}
		d.Plays = append(d.Plays, play)
	}
	return d
}

// validateChartBlocks は slack-go 自身の検証（Slack の制約の写し）を全ブロックに掛ける。
func validateChartBlocks(t *testing.T, msg focusMessage) {
	t.Helper()
	if len(msg.Blocks) > 2 {
		t.Fatalf("1 メッセージの data_visualization = %d 個, want <= 2", len(msg.Blocks))
	}
	for i, block := range msg.Blocks {
		viz, ok := block.(*slack.DataVisualizationBlock)
		if !ok {
			t.Fatalf("blocks[%d] = %T, want *slack.DataVisualizationBlock", i, block)
		}
		if err := viz.Validate(); err != nil {
			t.Fatalf("blocks[%d] が Slack の制約を満たさない: %v", i, err)
		}
		if n := utf8.RuneCountInString(viz.Title); n > focusChartTitleLimit {
			t.Fatalf("blocks[%d].Title = %d 文字, want <= %d", i, n, focusChartTitleLimit)
		}
	}
}

// #659 AC-2 / AC-5: pie（値 > 0）と bar（categories と点ラベルが一致・系列名が一意）が
// 1 通に収まり、通知用の text が空でない。
func TestFocusChartMessage_PieAndBar(t *testing.T) {
	msg, ok := focusChartMessage(rankThemes(chartFixture(), false))
	if !ok {
		t.Fatal("チャート通が組まれていない")
	}
	validateChartBlocks(t, msg)
	if len(msg.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2（pie と bar）", len(msg.Blocks))
	}

	pie, ok := msg.Blocks[0].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationPieChart)
	if !ok {
		t.Fatalf("1 個目が pie でない: %T", msg.Blocks[0].(*slack.DataVisualizationBlock).Chart)
	}
	if len(pie.Segments) > focusChartMaxSegments {
		t.Fatalf("pie のセグメント = %d, want <= %d", len(pie.Segments), focusChartMaxSegments)
	}
	for i, s := range pie.Segments {
		if s.Value <= 0 {
			t.Fatalf("pie.Segments[%d].Value = %v, want > 0", i, s.Value)
		}
		if n := utf8.RuneCountInString(s.Label); n > focusChartLabelLimit {
			t.Fatalf("pie.Segments[%d].Label = %d 文字, want <= %d", i, n, focusChartLabelLimit)
		}
	}

	bar, ok := msg.Blocks[1].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationBarChart)
	if !ok {
		t.Fatalf("2 個目が bar でない: %T", msg.Blocks[1].(*slack.DataVisualizationBlock).Chart)
	}
	// #681: x 軸はポジション（件数降順・同数は初出順）で、系列は 1 本。
	if got := strings.Join(bar.AxisConfig.Categories, ","); got != "QB,WR,OL,TE,RB" {
		t.Fatalf("bar の categories = %q, want ポジションの件数降順", got)
	}
	if len(bar.Series) != 1 || bar.Series[0].Name != focusChartPositionName {
		t.Fatalf("bar の系列 = %+v, want %q の 1 本", bar.Series, focusChartPositionName)
	}
	names := map[string]struct{}{}
	for _, s := range bar.Series {
		if _, dup := names[s.Name]; dup {
			t.Fatalf("系列名が重複している: %q", s.Name)
		}
		names[s.Name] = struct{}{}
		if len(s.Data) != len(bar.AxisConfig.Categories) {
			t.Fatalf("系列 %q の点数 = %d, want %d（0 埋めされていない）",
				s.Name, len(s.Data), len(bar.AxisConfig.Categories))
		}
		for j, p := range s.Data {
			if p.Label != bar.AxisConfig.Categories[j] {
				t.Fatalf("系列 %q の点 %d のラベル = %q, want %q", s.Name, j, p.Label, bar.AxisConfig.Categories[j])
			}
		}
	}

	if msg.Text == "" {
		t.Fatal("通知用の text が空（モバイル通知に何も出ない）")
	}
	if !strings.HasPrefix(msg.Text, "課題の内訳: ") || !strings.Contains(msg.Text, "%") {
		t.Fatalf("text = %q, want `課題の内訳: … %%` の 1 行", msg.Text)
	}
}

// #659 AC-3: テーマが多くても pie は focus に採った上位 ＋「その他」に畳む
// （slack-go v0.29.0 の Validate は 6 が上限。公式 reference は 12）。
func TestFocusChartMessage_ManyThemes(t *testing.T) {
	d := focusDigest{}
	for i := 0; i < 15; i++ {
		d.Themes = append(d.Themes, focusTheme{Key: fmt.Sprintf("t%02d", i), Title: fmt.Sprintf("テーマ%02d", i)})
	}
	// テーマ i を (15-i) 件のプレーに紐づけ、件数に差を付ける。
	for i := 0; i < 15; i++ {
		for n := 0; n < 15-i; n++ {
			d.Plays = append(d.Plays, focusPlay{
				Headline:  fmt.Sprintf("見出し%d", n%3),
				Name:      fmt.Sprintf("プレー%02d-%02d", i, n),
				ThemeKeys: []string{fmt.Sprintf("t%02d", i)},
				Positions: []string{"QB"},
			})
		}
	}

	report := rankThemes(d, false)
	msg, ok := focusChartMessage(report)
	if !ok {
		t.Fatal("チャート通が組まれていない")
	}
	validateChartBlocks(t, msg)

	pie := msg.Blocks[0].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationPieChart)
	if len(pie.Segments) != focusMaxThemes+1 {
		t.Fatalf("pie のセグメント = %d, want %d（focus %d ＋ その他）",
			len(pie.Segments), focusMaxThemes+1, focusMaxThemes)
	}
	if got := pie.Segments[len(pie.Segments)-1].Label; got != focusChartOtherLabel {
		t.Fatalf("末尾のセグメント = %q, want %q", got, focusChartOtherLabel)
	}
	// 畳んでも総数は失わない（15+14+…+1 = 120）。
	total := 0.0
	for _, s := range pie.Segments {
		total += s.Value
	}
	if total != 120 {
		t.Fatalf("セグメントの合計 = %v, want 120（その他に畳んでも数を失わない）", total)
	}
}

// #659 AC-4 / #681: ポジションが 1 種類なら bar を省略し pie のみ。
// 集計が空ならチャート通を出さない。
func TestFocusChartMessage_Degenerate(t *testing.T) {
	t.Run("ポジションが 1 種類なら pie のみ", func(t *testing.T) {
		d := focusDigest{
			Themes: []focusTheme{{Key: "a", Title: "A"}, {Key: "b", Title: "B"}},
			Plays: []focusPlay{
				{Headline: "skel", Name: "1", ThemeKeys: []string{"a"}, Positions: []string{"QB"}},
				{Headline: "GL", Name: "2", ThemeKeys: []string{"a", "b"}, Positions: []string{"QB"}},
				{Headline: "skel", Name: "3", ThemeKeys: []string{"b"}, Positions: []string{"QB"}},
			},
		}
		msg, ok := focusChartMessage(rankThemes(d, false))
		if !ok {
			t.Fatal("チャート通が組まれていない")
		}
		validateChartBlocks(t, msg)
		if len(msg.Blocks) != 1 {
			t.Fatalf("blocks = %d, want 1（ポジションが 1 種類なら bar を省く）", len(msg.Blocks))
		}
	})

	t.Run("集計が空ならチャートを出さない", func(t *testing.T) {
		if _, ok := focusChartMessage(focusReport{}); ok {
			t.Fatal("空の集計でチャート通を組んでいる（平文フォールバック経路）")
		}
	})
}

// #659 AC-5: 長いラベルは 20 文字に切り詰められ、空のポジションも捨てない。
func TestFocusChartMessage_Truncation(t *testing.T) {
	long := strings.Repeat("あ", 40)
	d := focusDigest{
		Themes: []focusTheme{{Key: "a", Title: long}, {Key: "b", Title: "B"}},
		Plays: []focusPlay{
			{Headline: long, Name: "1", ThemeKeys: []string{"a"}, Positions: []string{long}},
			{Headline: "", Name: "2", ThemeKeys: []string{"a", "b"}},
			{Headline: "skel", Name: "3", ThemeKeys: []string{"b"}, Positions: []string{"QB"}},
		},
	}
	msg, ok := focusChartMessage(rankThemes(d, false))
	if !ok {
		t.Fatal("チャート通が組まれていない")
	}
	validateChartBlocks(t, msg) // ラベル 20 文字超なら Validate が落ちる

	bar := msg.Blocks[1].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationBarChart)
	if !slices.Contains(bar.AxisConfig.Categories, focusUnknownPosition) {
		t.Fatalf("ポジション未特定が %q に寄せられていない: %v", focusUnknownPosition, bar.AxisConfig.Categories)
	}
	if !slices.Contains(bar.AxisConfig.Categories, truncateRunes(long, focusChartLabelLimit)) {
		t.Fatalf("長いポジションが切り詰められていない: %v", bar.AxisConfig.Categories)
	}
	// #681: pie の凡例は長い title ではなく短い label を使うので切り詰めが起きない。
	pie := msg.Blocks[0].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationPieChart)
	if got := pie.Segments[0].Label; got != truncateRunes(long, focusLabelRuneLimit) {
		t.Fatalf("pie のラベル = %q, want label（%d 文字に正規化済み）", got, focusLabelRuneLimit)
	}
}

// #681: ポジションが x 軸の上限を超えるぶんは「その他」に合算し、件数を失わない。
func TestFocusChartMessage_ManyPositions(t *testing.T) {
	const positions = focusChartMaxCategorie + 5
	d := focusDigest{Themes: []focusTheme{{Key: "a", Title: "A"}}}
	total := 0
	for i := 0; i < positions; i++ {
		for n := 0; n <= i; n++ { // POS{i} を i+1 件（件数に差を付ける）
			d.Plays = append(d.Plays, focusPlay{
				Headline:  "skel",
				Name:      fmt.Sprintf("プレー%02d-%02d", i, n),
				ThemeKeys: []string{"a"},
				Positions: []string{fmt.Sprintf("POS%02d", i)},
			})
			total++
		}
	}
	msg, ok := focusChartMessage(rankThemes(d, false))
	if !ok {
		t.Fatal("チャート通が組まれていない")
	}
	validateChartBlocks(t, msg)

	bar := msg.Blocks[1].(*slack.DataVisualizationBlock).Chart.(*slack.DataVisualizationBarChart)
	if len(bar.AxisConfig.Categories) != focusChartMaxCategorie {
		t.Fatalf("categories = %d, want %d（上限まで ＋ その他）",
			len(bar.AxisConfig.Categories), focusChartMaxCategorie)
	}
	if got := bar.AxisConfig.Categories[len(bar.AxisConfig.Categories)-1]; got != focusChartOtherLabel {
		t.Fatalf("末尾のカテゴリ = %q, want %q", got, focusChartOtherLabel)
	}
	if len(bar.Series) != 1 {
		t.Fatalf("系列 = %d, want 1（ポジションのみの集計）", len(bar.Series))
	}
	sum := 0.0
	for _, p := range bar.Series[0].Data {
		sum += p.Value
	}
	if sum != float64(total) {
		t.Fatalf("bar の合計 = %v, want %d（その他に合算しても数を失わない）", sum, total)
	}
}
