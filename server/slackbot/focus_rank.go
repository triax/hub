package slackbot

import (
	"slices"
	"sort"
	"strings"
)

// 焦点として採るテーマ数の上限。few（対象が少ない入力）はさらに絞る。
const (
	focusMaxThemes    = 5
	focusMaxThemesFew = 3
	// 代表プレー名として保持する件数。
	focusMaxSamplePlays = 3
	// ポジション・見出しが特定できなかったときの表示名。集計の穴を黙って捨てない。
	focusUnknownPosition = "不明"
	focusUnknownHeadline = "その他"
)

// rankedTheme は focus として採用されたテーマ 1 点。Count は実データから数えた
// 指摘プレー数で、LLM の自己申告ではない。
type rankedTheme struct {
	focusTheme
	Count int
	// Plays は代表プレー名（入力順で最大 focusMaxSamplePlays 件）。
	// stellar:debt(scope) 導出するだけで 1 通目には描いていない（focus を短く保つため）。
	// upgrade: 詳細スレッドで focus とプレーを相互リンクするか、不要なら導出ごと落とす
	Plays []string
}

// labelCount / themeCount は集計の 1 行。チャート（#659）が描画に使う。
type labelCount struct {
	Label string
	Count int
}

type themeCount struct {
	Key   string
	Title string
	Count int
}

// focusStats は描画に依らない集計結果。件数降順・同数は入力順で並ぶ（決定的）。
type focusStats struct {
	Themes    []themeCount
	Positions []labelCount
	// Headlines は見出しの初出順。HeadlinePositions は 見出し → ポジション → 件数。
	Headlines         []string
	HeadlinePositions map[string]map[string]int
}

// empty は描くべき中身が無いことを返す。
func (s focusStats) empty() bool { return len(s.Themes) == 0 && len(s.Positions) == 0 }

// focusReport は 1 通目・チャート・詳細のすべてが読む描画用の入力。
type focusReport struct {
	Focus []rankedTheme
	Plays []focusPlay
	Stats focusStats
}

// rankThemes は LLM が挙げたテーマ候補を、plays の theme_keys の実数で並べ替え、
// 上位を focus として採用する。件数 1 のテーマは focus に入れない（単発は詳細側の材料）が、
// それで 0 件になる入力では順位のまま採る（focus が空だと平文フォールバックに落ちるため）。
//
// 決定的であること（同じ入力 → 同じ出力、同数は themes の入力順）を保証する。
func rankThemes(d focusDigest, few bool) focusReport {
	counts := countThemeKeys(d)

	order := make([]int, 0, len(d.Themes))
	for i := range d.Themes {
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return counts[d.Themes[order[i]].Key] > counts[d.Themes[order[j]].Key]
	})

	limit := focusMaxThemes
	if few {
		limit = focusMaxThemesFew
	}
	focus := pickThemes(d, order, counts, limit, 2)
	if len(focus) == 0 {
		// 全テーマが 1 件以下。0 件にするより順位のまま見せるほうが役に立つ。
		focus = pickThemes(d, order, counts, limit, 1)
	}

	return focusReport{Focus: focus, Plays: d.Plays, Stats: buildFocusStats(d, order, counts)}
}

// pickThemes は order の順に minCount 件以上のテーマを limit 件まで採る。
func pickThemes(d focusDigest, order []int, counts map[string]int, limit, minCount int) []rankedTheme {
	picked := make([]rankedTheme, 0, limit)
	for _, i := range order {
		theme := d.Themes[i]
		count := counts[theme.Key]
		if count < minCount || theme.Key == "" {
			continue
		}
		picked = append(picked, rankedTheme{
			focusTheme: theme,
			Count:      count,
			Plays:      samplePlays(d.Plays, theme.Key),
		})
		if len(picked) == limit {
			break
		}
	}
	return picked
}

// countThemeKeys は「そのテーマを指しているプレー数」を数える。1 プレーが同じ key を
// 重複して挙げても 1 件として数える（LLM の重複で件数が水増しされないように）。
func countThemeKeys(d focusDigest) map[string]int {
	counts := make(map[string]int, len(d.Themes))
	for _, play := range d.Plays {
		for _, key := range uniqueStrings(play.ThemeKeys) {
			counts[key]++
		}
	}
	return counts
}

func samplePlays(plays []focusPlay, key string) []string {
	names := make([]string, 0, focusMaxSamplePlays)
	for _, play := range plays {
		if play.Name == "" || !slices.Contains(play.ThemeKeys, key) {
			continue
		}
		names = append(names, play.Name)
		if len(names) == focusMaxSamplePlays {
			break
		}
	}
	return names
}

// buildFocusStats は チャート（#659）が読む集計を組む。テーマは order（件数降順・
// 同数は入力順）、ポジションは件数降順・同数は初出順、見出しは初出順。
func buildFocusStats(d focusDigest, order []int, counts map[string]int) focusStats {
	stats := focusStats{HeadlinePositions: map[string]map[string]int{}}

	for _, i := range order {
		theme := d.Themes[i]
		if theme.Key == "" || counts[theme.Key] == 0 {
			continue
		}
		stats.Themes = append(stats.Themes, themeCount{Key: theme.Key, Title: theme.Title, Count: counts[theme.Key]})
	}

	positions := map[string]int{}
	positionOrder := []string{}
	for _, play := range d.Plays {
		headline := focusHeadlineLabel(play.Headline)
		if _, seen := stats.HeadlinePositions[headline]; !seen {
			stats.HeadlinePositions[headline] = map[string]int{}
			stats.Headlines = append(stats.Headlines, headline)
		}
		for _, position := range playPositions(play) {
			if _, seen := positions[position]; !seen {
				positionOrder = append(positionOrder, position)
			}
			positions[position]++
			stats.HeadlinePositions[headline][position]++
		}
	}
	stats.Positions = sortedCounts(positionOrder, positions)
	return stats
}

// focusHeadlineLabel は 1 プレーが属する見出しの表示名。空欄の寄せ先を 1 箇所に
// 閉じることで、チャートの x 軸（buildFocusStats）と詳細の見出し（groupPlaysByHeadline）が
// 同じ区切りを指すことを保証する。
func focusHeadlineLabel(headline string) string {
	if h := strings.TrimSpace(headline); h != "" {
		return h
	}
	return focusUnknownHeadline
}

// playPositions は 1 プレーのポジション（重複除去・空なら「不明」）を返す。
func playPositions(play focusPlay) []string {
	kept := uniqueStrings(play.Positions)
	if len(kept) == 0 {
		return []string{focusUnknownPosition}
	}
	return kept
}

// sortedCounts は件数降順・同数は order（初出順）で並べる。
func sortedCounts(order []string, counts map[string]int) []labelCount {
	out := make([]labelCount, 0, len(order))
	for _, label := range order {
		out = append(out, labelCount{Label: label, Count: counts[label]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	kept := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		kept = append(kept, v)
	}
	return kept
}
