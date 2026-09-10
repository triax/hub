package slackbot

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// pie / bar のタイトル。数字は**未来の予測ではなく過去の実測**（反省スレッドでの指摘件数）
// なので、「崩れどころの内訳」のような書き方にすると「この割合で崩れる」と読める。
// 何を数えたのかをタイトルで明示する（#683）。
const (
	premortemPieTitle = "根拠になった指摘の内訳"
	premortemBarTitle = "ポジション別の該当指摘数"
)

// premortemChartMessage は集計を pie / bar の 1 通に組む。
// 集計が空（平文フォールバック経路）なら false を返し、チャートを出さない。
func premortemChartMessage(report premortemReport) (premortemMessage, bool) {
	if report.Stats.empty() {
		return premortemMessage{}, false
	}
	segments := riskSegments(report)
	if len(segments) == 0 {
		return premortemMessage{}, false
	}

	blocks := []slack.Block{slack.NewDataVisualizationBlock(
		truncateRunes(premortemPieTitle, focusChartTitleLimit),
		slack.NewDataVisualizationPieChart(segments...),
		slack.DataVisualizationBlockOptionBlockID("premortem_chart_risks"),
	)}
	// ポジションが 1 種類しか無い（スレッド単体など）と比較にならないので bar は出さない。
	if len(report.Stats.Positions) > 1 {
		if categories, series := premortemPositionSeries(report.Stats); len(series) > 0 {
			blocks = append(blocks, slack.NewDataVisualizationBlock(
				truncateRunes(premortemBarTitle, focusChartTitleLimit),
				slack.NewDataVisualizationBarChart(slack.NewDataVisualizationAxisConfig(categories...), series...),
				slack.DataVisualizationBlockOptionBlockID("premortem_chart_positions"),
			))
		}
	}
	return premortemMessage{Text: premortemChartFallbackText(segments), Blocks: blocks}, true
}

// riskSegments は pie のセグメント。採用した負け筋をその順で並べ、残り（採用外の候補）は
// 「その他」に畳む。ラベルは短い label（長い title は凡例で二重に切り詰められる。#681）。
func riskSegments(report premortemReport) []slack.DataVisualizationSegment {
	adopted := make(map[string]struct{}, len(report.Risks))
	segments := make([]slack.DataVisualizationSegment, 0, focusChartMaxSegments)
	for _, r := range report.Risks {
		if r.Count <= 0 {
			break // Risks は件数降順。0 件が出たら以降も 0 件
		}
		adopted[r.Key] = struct{}{}
		segments = append(segments, slack.NewDataVisualizationSegment(
			truncateRunes(chartLabel(r.Label, r.Key), focusChartLabelLimit), float64(r.Count)))
	}

	rest := 0
	for _, risk := range report.Stats.Risks {
		if _, ok := adopted[risk.Key]; !ok {
			rest += risk.Count
		}
	}
	if rest > 0 {
		segments = append(segments, slack.NewDataVisualizationSegment(focusChartOtherLabel, float64(rest)))
	}
	return segments
}

// premortemPositionSeries は bar の x 軸（ポジション）と 1 本の系列を組む。
func premortemPositionSeries(stats premortemStats) ([]string, []slack.DataVisualizationDataSeries) {
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

func premortemChartFallbackText(segments []slack.DataVisualizationSegment) string {
	total := 0.0
	for _, s := range segments {
		total += s.Value
	}
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		parts = append(parts, fmt.Sprintf("%s %.0f%%", s.Label, s.Value/total*100))
	}
	return premortemPieTitle + ": " + strings.Join(parts, " / ")
}
