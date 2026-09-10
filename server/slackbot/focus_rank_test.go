package slackbot

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// rankFixture は #658 AC-1 の入力: テーマ 5・プレー 20、theme_keys の分布は
// 8 / 6 / 3 / 2 / 1。テーマの入力順は件数順と一致させない（並べ替えの検証のため）。
func rankFixture() focusDigest {
	d := focusDigest{Themes: []focusTheme{
		{Key: "solo", Title: "単発の指摘", Positions: []string{"OL"}},        // 1 件
		{Key: "timing", Title: "リリースのタイミング"},                            // 8 件
		{Key: "depth", Title: "ステムの深さ", Positions: []string{"WR"}},      // 2 件
		{Key: "read", Title: "セーフティの読み", Positions: []string{"QB"}},     // 6 件
		{Key: "stance", Title: "スタンス", Positions: []string{"OL", "TE"}}, // 3 件
	}}
	// key ごとの出現数を作る。1 プレーが複数テーマに属するので合計は 20 を超える。
	plan := []struct {
		key string
		n   int
	}{{"timing", 8}, {"read", 6}, {"stance", 3}, {"depth", 2}, {"solo", 1}}
	for i := 0; i < 20; i++ {
		play := focusPlay{
			Headline:  fmt.Sprintf("見出し%d", i%4),
			Name:      fmt.Sprintf("プレー%02d", i),
			Positions: []string{[]string{"QB", "WR", "OL", "TE", "RB"}[i%5]},
			Issue:     "指摘",
		}
		for _, p := range plan {
			if i < p.n {
				play.ThemeKeys = append(play.ThemeKeys, p.key)
			}
		}
		d.Plays = append(d.Plays, play)
	}
	return d
}

// #658 AC-1: 件数順に並び、件数 1 のテーマは focus に入らず、count は theme_keys の実数。
func TestRankThemes(t *testing.T) {
	report := rankThemes(rankFixture(), false)

	// #681: focusMaxThemes を 5 → 3 に絞ったので、件数上位 3 件だけが採られる
	// （件数 1 の solo は元から除外。件数 2 の depth は上限からあふれる）。
	if len(report.Focus) != focusMaxThemes {
		t.Fatalf("focus = %d 件, want %d（上位のみ）", len(report.Focus), focusMaxThemes)
	}
	wantKeys := []string{"timing", "read", "stance"}
	wantCounts := []int{8, 6, 3}
	for i := range wantKeys {
		if got := report.Focus[i].Key; got != wantKeys[i] {
			t.Fatalf("focus[%d].Key = %q, want %q（件数順に並んでいない）", i, got, wantKeys[i])
		}
		if got := report.Focus[i].Count; got != wantCounts[i] {
			t.Fatalf("focus[%d].Count = %d, want %d", i, got, wantCounts[i])
		}
	}
	for i, f := range report.Focus {
		if len(f.Plays) > focusMaxSamplePlays {
			t.Fatalf("focus[%d].Plays = %d 件, want <= %d", i, len(f.Plays), focusMaxSamplePlays)
		}
	}
	if got := strings.Join(report.Focus[0].Plays, ","); got != "プレー00,プレー01,プレー02" {
		t.Fatalf("代表プレーが入力順の先頭 3 件でない: %q", got)
	}
	if len(report.Plays) != 20 {
		t.Fatalf("plays = %d, want 20（そのまま持ち回る）", len(report.Plays))
	}
}

// #658 AC-2 / #681: few（ThreadOnly / 返信付き投稿が少ない）では focus を 1 件に絞る。
func TestRankThemes_Few(t *testing.T) {
	report := rankThemes(rankFixture(), true)
	if focusMaxThemesFew != 1 {
		t.Fatalf("focusMaxThemesFew = %d, want 1（対象が少ない入力は 1 点に振り切る）", focusMaxThemesFew)
	}
	if len(report.Focus) != focusMaxThemesFew {
		t.Fatalf("focus = %d 件, want %d", len(report.Focus), focusMaxThemesFew)
	}
	if report.Focus[0].Key != "timing" {
		t.Fatalf("focus[0].Key = %q, want timing", report.Focus[0].Key)
	}
}

// 同じ入力からは常に同じ順位・件数が出る（map の反復順に引きずられない）。
func TestRankThemes_Deterministic(t *testing.T) {
	first := rankThemes(rankFixture(), false)
	for i := 0; i < 20; i++ {
		if got := rankThemes(rankFixture(), false); !reflect.DeepEqual(got, first) {
			t.Fatalf("集計が実行ごとにぶれている:\n1 回目: %+v\n%d 回目: %+v", first, i+2, got)
		}
	}
}

// 同数のテーマは themes の入力順で並ぶ。
func TestRankThemes_TieKeepsInputOrder(t *testing.T) {
	d := focusDigest{
		Themes: []focusTheme{{Key: "b", Title: "B"}, {Key: "a", Title: "A"}, {Key: "c", Title: "C"}},
		Plays: []focusPlay{
			{Name: "1", ThemeKeys: []string{"a", "b", "c"}},
			{Name: "2", ThemeKeys: []string{"a", "b", "c"}},
		},
	}
	report := rankThemes(d, false)
	if got := fmt.Sprintf("%s,%s,%s", report.Focus[0].Key, report.Focus[1].Key, report.Focus[2].Key); got != "b,a,c" {
		t.Fatalf("同数の並び = %q, want b,a,c（入力順）", got)
	}
}

// 全テーマが件数 1 でも focus を 0 件にしない（0 件は平文フォールバックに落ちるため）。
func TestRankThemes_AllSingletons(t *testing.T) {
	d := focusDigest{
		Themes: []focusTheme{{Key: "a", Title: "A"}, {Key: "b", Title: "B"}},
		Plays: []focusPlay{
			{Name: "1", ThemeKeys: []string{"a"}},
			{Name: "2", ThemeKeys: []string{"b"}},
		},
	}
	report := rankThemes(d, false)
	if len(report.Focus) != 2 {
		t.Fatalf("focus = %d 件, want 2（全テーマが件数 1 なら順位のまま採る）", len(report.Focus))
	}
	if report.Focus[0].Count != 1 {
		t.Fatalf("focus[0].Count = %d, want 1", report.Focus[0].Count)
	}
}

// 同じプレーが同じ key を重複して挙げても 1 件として数える（件数の水増しを防ぐ）。
func TestRankThemes_DedupesThemeKeysPerPlay(t *testing.T) {
	d := focusDigest{
		Themes: []focusTheme{{Key: "a", Title: "A"}},
		Plays:  []focusPlay{{Name: "1", ThemeKeys: []string{"a", "a", "a"}}},
	}
	if got := rankThemes(d, false).Focus[0].Count; got != 1 {
		t.Fatalf("Count = %d, want 1（1 プレーは 1 件）", got)
	}
}

// #659 が読む集計。ポジションは件数降順で、空欄は「不明」に寄せる。
func TestBuildFocusStats(t *testing.T) {
	d := focusDigest{
		Themes: []focusTheme{{Key: "a", Title: "A"}, {Key: "b", Title: "B"}, {Key: "z", Title: "Z"}},
		Plays: []focusPlay{
			{Headline: "skel", Name: "1", ThemeKeys: []string{"a"}, Positions: []string{"QB", "WR"}},
			{Headline: "skel", Name: "2", ThemeKeys: []string{"a", "b"}, Positions: []string{"QB"}},
			{Headline: "", Name: "3", ThemeKeys: []string{"b"}, Positions: nil},
		},
	}
	stats := rankThemes(d, false).Stats

	// 参照ゼロの z は Themes に出さない（pie に値 0 のセグメントを作らないため）。
	want := []themeCount{{Key: "a", Count: 2}, {Key: "b", Count: 2}}
	if !reflect.DeepEqual(stats.Themes, want) {
		t.Fatalf("Stats.Themes = %+v, want %+v", stats.Themes, want)
	}
	wantPositions := []labelCount{{Label: "QB", Count: 2}, {Label: "WR", Count: 1}, {Label: focusUnknownPosition, Count: 1}}
	if !reflect.DeepEqual(stats.Positions, wantPositions) {
		t.Fatalf("Stats.Positions = %+v, want %+v", stats.Positions, wantPositions)
	}
	if stats.empty() {
		t.Fatal("中身のある集計が empty 判定になっている")
	}
	if !(focusStats{}).empty() {
		t.Fatal("空の集計が empty 判定になっていない")
	}
}

// #661 AC-2: LLM が挙げた「やる」「やめる」は、空文字と重複を落として各 2 件に
// 切り詰める（件数は Structured Outputs の strict schema では縛れない）。
// do が 0 件でも focus からは外さない。
func TestNormalizeTheme_TrimsActions(t *testing.T) {
	theme := normalizeTheme(focusTheme{
		Key:  "timing",
		Do:   []string{" ブレイク 3 歩目で離す ", "", "ブレイク 3 歩目で離す", "MOFO/MOFC を先に決める", "フラットは最後に読む"},
		Dont: []string{"   "},
	})
	if got := strings.Join(theme.Do, "|"); got != "ブレイク 3 歩目で離す|MOFO/MOFC を先に決める" {
		t.Fatalf("Do = %q, want 空文字・重複を除いた先頭 %d 件", got, focusMaxActions)
	}
	if len(theme.Dont) != 0 {
		t.Fatalf("Dont = %+v, want 空（空白だけの要素は落とす）", theme.Dont)
	}

	// #668 AC-1: テーマ側 Positions も重複・空文字・前後空白を除く。
	positioned := normalizeTheme(focusTheme{
		Key:       "targets",
		Positions: []string{"QB", "QB", " WR ", ""},
	})
	if got := strings.Join(positioned.Positions, ","); got != "QB,WR" {
		t.Fatalf("Positions = %q, want QB,WR（重複・空文字・前後空白を除く）", got)
	}
	if got := focusItemMeta(rankedTheme{focusTheme: positioned}, 0); !strings.Contains(got, "QB, WR") {
		t.Fatalf("補足行 = %q, want QB, WR を含む", got)
	}

	// #668 AC-2: Title / Summary / Quote の前後空白も normalizeTheme に集約される。
	trimmed := normalizeTheme(focusTheme{
		Key:     "spacing",
		Title:   "  タイトル  ",
		Summary: "  概要  ",
		Quote:   "  原文の引用  ",
	})
	if trimmed.Title != "タイトル" {
		t.Fatalf("Title = %q, want トリム済み", trimmed.Title)
	}
	if trimmed.Summary != "概要" {
		t.Fatalf("Summary = %q, want トリム済み", trimmed.Summary)
	}
	if trimmed.Quote != "原文の引用" {
		t.Fatalf("Quote = %q, want トリム済み", trimmed.Quote)
	}

	// rankThemes 経由でも同じで、do 0 件のテーマが focus から消えない。
	d := focusDigest{
		Themes: []focusTheme{{Key: "a", Title: "A", Do: []string{"やること", "", "やること"}, Dont: nil}},
		Plays:  []focusPlay{{Name: "1", ThemeKeys: []string{"a"}}, {Name: "2", ThemeKeys: []string{"a"}}},
	}
	focus := rankThemes(d, false).Focus
	if len(focus) != 1 {
		t.Fatalf("focus = %d 件, want 1", len(focus))
	}
	if got := strings.Join(focus[0].Do, "|"); got != "やること" {
		t.Fatalf("focus[0].Do = %q, want 重複を除いた 1 件", got)
	}
	if len(focus[0].Dont) != 0 {
		t.Fatalf("focus[0].Dont = %+v, want 空（やめるが無くても focus に残る）", focus[0].Dont)
	}
}

// #681 AC-3: label は trim して 14 文字に収め、空なら title の切り詰めに倒す
// （strict schema は「必ず埋める」までは縛れないので、ここが唯一の関門）。
func TestNormalizeTheme_Label(t *testing.T) {
	long := strings.Repeat("あ", 40)
	for _, c := range []struct {
		name  string
		theme focusTheme
		want  string
	}{
		{"前後空白を落とす", focusTheme{Label: "  ブレイク前後の減速  "}, "ブレイク前後の減速"},
		{"長すぎる label は切り詰める", focusTheme{Label: long}, truncateRunes(long, focusLabelRuneLimit)},
		{"空なら title に倒す", focusTheme{Title: "  QB↔WR のタイミング  "}, "QB↔WR のタイミング"},
		{"空白だけでも title に倒す", focusTheme{Label: "   ", Title: long}, truncateRunes(long, focusLabelRuneLimit)},
		{"title も空なら空のまま", focusTheme{}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeTheme(c.theme).Label; got != c.want {
				t.Fatalf("Label = %q, want %q", got, c.want)
			}
		})
	}
}
