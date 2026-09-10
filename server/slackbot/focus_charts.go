package slackbot

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Slack の data_visualization ブロックの上限。1 メッセージ 2 個まで、title 50 文字、
// ラベル 20 文字。セグメント数は公式 reference が 12、slack-go v0.29.0 の Validate() が
// 6 を上限とするので、両方の解釈で安全な 6 に倒す
// （focus は最大 3 テーマなので実データ上の欠落も起きない）。
const (
	focusChartMaxSegments  = 6
	focusChartMaxCategorie = 20 // 系列あたりのデータ点上限（= x 軸のカテゴリ数上限）
	focusChartLabelLimit   = 20
	focusChartTitleLimit   = 50
	focusChartOtherLabel   = "その他"
	focusChartPositionName = "指摘プレー数"
)

// pie は focus に採られたテーマを 1 つ残らず描き、残りを「その他」の 1 枠に畳む。
// それが成り立つのは focus の上限がセグメント上限の内側にあるときだけなので、
// 定数どうしの関係をコンパイル時に縛る（focusMaxThemes を増やしたらここで落ちる）。
const _ = uint(focusChartMaxSegments - 1 - focusMaxThemes)

// focusChartMessage は集計（focusStats）を pie / bar の 1 通に組む。
// 集計が空（平文フォールバック経路）なら false を返し、チャートを出さない。
func focusChartMessage(report focusReport) (focusMessage, bool) {
	if report.Stats.empty() {
		return focusMessage{}, false
	}
	segments := themeSegments(report)
	if len(segments) == 0 {
		return focusMessage{}, false
	}

	blocks := []slack.Block{slack.NewDataVisualizationBlock(
		truncateRunes("課題の内訳", focusChartTitleLimit),
		slack.NewDataVisualizationPieChart(segments...),
		slack.DataVisualizationBlockOptionBlockID("focus_chart_themes"),
	)}
	// ポジションが 1 種類しか無い（スレッド単体など）と比較にならないので bar は出さない。
	categories, series := positionSeries(report.Stats)
	if len(report.Stats.Positions) > 1 && len(series) > 0 {
		blocks = append(blocks, slack.NewDataVisualizationBlock(
			truncateRunes("ポジション別の指摘数", focusChartTitleLimit),
			slack.NewDataVisualizationBarChart(slack.NewDataVisualizationAxisConfig(categories...), series...),
			slack.DataVisualizationBlockOptionBlockID("focus_chart_positions"),
		))
	}
	return focusMessage{Text: chartFallbackText(segments), Blocks: blocks}, true
}

// themeSegments は pie のセグメント。focus に採られたテーマをその順で並べ、
// 残りのテーマ（件数 1 の単発を含む）は「その他」に畳む。ラベルは短い label で、
// 長い title を使うと focusChartLabelLimit と Slack の凡例で二重に切り詰められる（#681）。
func themeSegments(report focusReport) []slack.DataVisualizationSegment {
	focused := make(map[string]struct{}, len(report.Focus))
	segments := make([]slack.DataVisualizationSegment, 0, focusChartMaxSegments)
	for _, f := range report.Focus {
		if f.Count <= 0 {
			break // Focus は件数降順。0 件が出たら以降も 0 件
		}
		focused[f.Key] = struct{}{}
		segments = append(segments, slack.NewDataVisualizationSegment(
			truncateRunes(chartLabel(f.Label, f.Key), focusChartLabelLimit), float64(f.Count)))
	}

	rest := 0
	for _, theme := range report.Stats.Themes {
		if _, ok := focused[theme.Key]; !ok {
			rest += theme.Count
		}
	}
	if rest > 0 {
		segments = append(segments, slack.NewDataVisualizationSegment(focusChartOtherLabel, float64(rest)))
	}
	return segments
}

// positionSeries は bar の x 軸（ポジション）と 1 本の系列を組む。見出し（練習日や
// ドリル）を x 軸に取るとボリュームが桁違いになり比較として成立しないので、
// ポジションだけで集計する（#681）。stats.Positions は件数降順・同数は初出順に
// 整列済みなので、そのまま並べる。
func positionSeries(stats focusStats) ([]string, []slack.DataVisualizationDataSeries) {
	// カテゴリ数の上限を超えるぶんは「その他」に合算する（末尾を切り捨てて数を失わない）。
	named := stats.Positions
	other := 0.0
	if len(named) > focusChartMaxCategorie {
		named = stats.Positions[:focusChartMaxCategorie-1]
		for _, position := range stats.Positions[focusChartMaxCategorie-1:] {
			other += float64(position.Count)
		}
	}

	categories := make([]string, 0, focusChartMaxCategorie)
	counts := make([]float64, 0, focusChartMaxCategorie)
	seen := map[string]struct{}{}
	for _, position := range named {
		label := truncateRunes(chartLabel(position.Label, focusUnknownPosition), focusChartLabelLimit)
		if _, ok := seen[label]; ok {
			continue // 切り詰めで衝突したポジションは捨てる（categories は一意でなければならない）
		}
		seen[label] = struct{}{}
		categories = append(categories, label)
		counts = append(counts, float64(position.Count))
	}
	if other > 0 {
		categories = append(categories, focusChartOtherLabel)
		counts = append(counts, other)
	}
	if len(categories) == 0 {
		return nil, nil
	}
	return categories, []slack.DataVisualizationDataSeries{
		slack.NewDataVisualizationDataSeries(focusChartPositionName, dataPoints(categories, counts)...)}
}

// dataPoints は各カテゴリに 1 点ずつ（出現しない組み合わせは 0）を置く。
// Slack は「系列ごとに全カテゴリぶんの点」を要求するので 0 を省略できない。
func dataPoints(categories []string, counts []float64) []slack.DataVisualizationDataPoint {
	points := make([]slack.DataVisualizationDataPoint, 0, len(categories))
	for j, label := range categories {
		points = append(points, slack.NewDataVisualizationDataPoint(label, counts[j]))
	}
	return points
}

// chartFallbackText はモバイル通知・検索に出る 1 行（blocks だけだと空になる）。
func chartFallbackText(segments []slack.DataVisualizationSegment) string {
	total := 0.0
	for _, s := range segments {
		total += s.Value
	}
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", s.Label, s.Value/total*100))
	}
	return "課題の内訳: " + strings.Join(parts, " / ")
}

// chartLabel は空ラベルを避ける。Slack はラベル 1 文字以上を要求する。
func chartLabel(value, fallback string) string {
	if v := strings.TrimSpace(value); v != "" {
		return v
	}
	return fallback
}
