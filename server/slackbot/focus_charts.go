package slackbot

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// Slack の data_visualization ブロックの上限。1 メッセージ 2 個まで、title 50 文字、
// ラベル 20 文字。セグメント数・系列数は公式 reference が 12、slack-go v0.29.0 の
// Validate() が 6 を上限とするので、両方の解釈で安全な 6 に倒す
// （focus は最大 5 テーマなので実データ上の欠落も起きない）。
const (
	focusChartMaxSegments  = 6
	focusChartMaxSeries    = 6
	focusChartMaxCategorie = 20 // 系列あたりのデータ点上限（= x 軸のカテゴリ数上限）
	focusChartLabelLimit   = 20
	focusChartTitleLimit   = 50
	focusChartOtherLabel   = "その他"
)

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
	// 見出しが 1 つしか無い（スレッド単体など）と場面の比較にならないので bar は出さない。
	if categories, series := positionSeries(report.Stats); len(categories) > 1 && len(series) > 0 {
		blocks = append(blocks, slack.NewDataVisualizationBlock(
			truncateRunes("ポジション × 場面", focusChartTitleLimit),
			slack.NewDataVisualizationBarChart(slack.NewDataVisualizationAxisConfig(categories...), series...),
			slack.DataVisualizationBlockOptionBlockID("focus_chart_positions"),
		))
	}
	return focusMessage{Text: chartFallbackText(segments), Blocks: blocks}, true
}

// themeSegments は pie のセグメント。focus に採られたテーマをその順で並べ、
// 残りのテーマ（件数 1 の単発を含む）は「その他」に畳む。
func themeSegments(report focusReport) []slack.DataVisualizationSegment {
	focused := make(map[string]struct{}, len(report.Focus))
	segments := make([]slack.DataVisualizationSegment, 0, focusChartMaxSegments)
	for _, f := range report.Focus {
		if f.Count <= 0 || len(segments) >= focusChartMaxSegments-1 {
			break
		}
		focused[f.Key] = struct{}{}
		segments = append(segments, slack.NewDataVisualizationSegment(
			truncateRunes(chartLabel(f.Title, f.Key), focusChartLabelLimit), float64(f.Count)))
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

// positionSeries は bar の x 軸（見出し）と系列（ポジション）を組む。Slack は
// 「系列ごとに全カテゴリぶんの点」を要求するので、出現しない組み合わせは 0 で埋める。
func positionSeries(stats focusStats) ([]string, []slack.DataVisualizationDataSeries) {
	categories := make([]string, 0, focusChartMaxCategorie)
	sources := make([]string, 0, focusChartMaxCategorie) // 切り詰め前の見出し（集計の引き当て用）
	seen := map[string]struct{}{}
	for _, headline := range stats.Headlines {
		if len(categories) == focusChartMaxCategorie {
			break
		}
		label := truncateRunes(chartLabel(headline, focusChartOtherLabel), focusChartLabelLimit)
		if _, ok := seen[label]; ok {
			continue // 切り詰めで衝突した見出しは捨てる（categories は一意でなければならない）
		}
		seen[label] = struct{}{}
		categories = append(categories, label)
		sources = append(sources, headline)
	}

	// 系列数の上限を超えるぶんは「その他」に合算する（末尾を切り捨てて数を失わない）。
	named := stats.Positions
	other := make([]float64, len(categories))
	hasOther := len(named) > focusChartMaxSeries
	if hasOther {
		named = stats.Positions[:focusChartMaxSeries-1]
		for _, position := range stats.Positions[focusChartMaxSeries-1:] {
			for j, headline := range sources {
				other[j] += float64(stats.HeadlinePositions[headline][position.Label])
			}
		}
	}

	series := make([]slack.DataVisualizationDataSeries, 0, focusChartMaxSeries)
	for _, position := range named {
		counts := make([]float64, len(categories))
		for j, headline := range sources {
			counts[j] = float64(stats.HeadlinePositions[headline][position.Label])
		}
		series = append(series, slack.NewDataVisualizationDataSeries(
			truncateRunes(position.Label, focusChartLabelLimit), dataPoints(categories, counts)...))
	}
	if hasOther {
		series = append(series, slack.NewDataVisualizationDataSeries(
			focusChartOtherLabel, dataPoints(categories, other)...))
	}
	return categories, series
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
