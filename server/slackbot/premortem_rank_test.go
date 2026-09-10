package slackbot

import (
	"strings"
	"testing"
)

// premortemRiskFixture は kind の異なる 4 候補。件数は plays 側で決まる。
//
//	ol_slide  execution   3 プレー
//	run_read  hypothesis  2 プレー
//	late_call adjust      2 プレー
//	extra     execution   1 プレー（minCount=2 に届かず採用外）
func premortemRiskFixture() premortemDigest {
	return premortemDigest{
		Risks: []premortemRisk{
			{
				Key: "ol_slide", Kind: premortemKindExecution,
				Title: "3rd&long で OL のスライドが間に合わず QB がポケット内で捕まる",
				Label: "3rd&long の被サック",
				Scenario: "直近 3 週の skel で 3rd&long のブリッツに対しスライドの合図が遅れている。" +
					"パスプロが 2.5 秒持たず、QB が第 2 選択に行く前に潰される。",
				Phase: "3rd&long", Unit: "OF",
				Signal:    "1Q で 3rd&long が 2 回続けて 5 ヤード以上のロスになったら、ホットルートに切り替える",
				Prevent:   []string{"OL: 水曜の skel でスライド合図を C からの 1 声に統一する", "QB: プレッシャー時のホットを 1 本決めておく"},
				Positions: []string{"OL", "QB"},
				Quote:     "スライドのコール遅れてQBが待ちすぎ",
			},
			{
				Key: "run_read", Kind: premortemKindHypothesis,
				Title:    "相手を「パス主体」と想定して守り、インサイドランで時間を削られる",
				Label:    "相手のラン想定違い",
				Scenario: "Slack にある相手の直近 2 試合の資料はいずれもパス偏重だが、それは点差がついた後の展開。",
				Phase:    "前半の 1st down", Unit: "DF",
				Signal:    "最初の 2 シリーズで相手がランを 6 回以上入れてきたら、こちらの想定が外れている",
				Prevent:   []string{"DL/LB: 火曜にインサイドランの基本セットを 1 本残す"},
				Positions: []string{"DL", "LB"},
				Quote:     "相手の資料パスばっかりだけど点差ついてからだよね",
			},
			{
				Key: "late_call", Kind: premortemKindAdjust,
				Title:    "相手の想定外セットに誰も気づかず、ハーフタイムまで修正が入らない",
				Label:    "修正コールの遅れ",
				Scenario: "練習試合でも「気づいてはいたが誰が言うか決まっていない」が繰り返し出ている。",
				Phase:    "1Q〜ハーフタイム", Unit: "DF",
				Signal:    "同じフォーメーションで 2 回連続ゲインされたら、その場でサイドラインに上げる",
				Prevent:   []string{"サイドライン: 相手セットを見る担当を DB コーチに固定する", "ハーフタイム前に修正 1 本を口頭で共有する"},
				Positions: []string{"DB", "LB"},
				Quote:     "気づいてたけど誰が言うか決まってなかった",
			},
			{
				Key: "extra", Kind: premortemKindExecution,
				Title: "スクリーンのリリースが遅れる", Label: "スクリーンの遅れ",
				Prevent: []string{"WR: リリースを 1 テンポ早くする"}, Positions: []string{"WR"},
			},
		},
		Plays: []premortemPlay{
			{Name: "P1", RiskKeys: []string{"ol_slide"}},
			{Name: "P2", RiskKeys: []string{"ol_slide", "run_read"}},
			{Name: "P3", RiskKeys: []string{"ol_slide", "late_call"}},
			{Name: "P4", RiskKeys: []string{"run_read"}},
			{Name: "P5", RiskKeys: []string{"late_call"}},
			{Name: "P6", RiskKeys: []string{"extra"}},
		},
	}
}

func riskKeys(risks []rankedRisk) string {
	keys := make([]string, 0, len(risks))
	for _, r := range risks {
		keys = append(keys, r.Key)
	}
	return strings.Join(keys, ",")
}

// AC-6 / AC-9: 順位は plays[].risk_keys の実数（降順・同数は入力順）で決まる。
func TestRankRisks_CountsFromPlays(t *testing.T) {
	report := rankRisks(premortemRiskFixture(), false)

	if got := riskKeys(report.Risks); got != "ol_slide,run_read,late_call" {
		t.Fatalf("risks = %q, want ol_slide,run_read,late_call", got)
	}
	wantCounts := []int{3, 2, 2}
	for i, r := range report.Risks {
		if r.Count != wantCounts[i] {
			t.Fatalf("risks[%d].Count = %d, want %d", i, r.Count, wantCounts[i])
		}
	}
	// stats は採用外（extra, 1 件）も含む。pie の「その他」がここから出る。
	if total := totalRiskCount(report.Stats); total != 8 {
		t.Fatalf("total = %d, want 8", total)
	}
}

// AC-30: 同じ kind が 2 件以上入らない。件数が多くても execution は 1 枠。
func TestRankRisks_KindDiversity(t *testing.T) {
	d := premortemDigest{
		Risks: []premortemRisk{
			{Key: "e1", Kind: premortemKindExecution, Title: "実行1", Label: "実行1"},
			{Key: "e2", Kind: premortemKindExecution, Title: "実行2", Label: "実行2"},
			{Key: "e3", Kind: premortemKindExecution, Title: "実行3", Label: "実行3"},
			{Key: "e4", Kind: premortemKindExecution, Title: "実行4", Label: "実行4"},
			{Key: "h1", Kind: premortemKindHypothesis, Title: "想定違い", Label: "想定違い"},
		},
		Plays: []premortemPlay{
			// 件数は e1 > e2 > e3 > e4 > h1。素直に件数順だと 3 点とも execution になる。
			{RiskKeys: []string{"e1", "e2", "e3", "e4", "h1"}},
			{RiskKeys: []string{"e1", "e2", "e3", "e4", "h1"}},
			{RiskKeys: []string{"e1", "e2", "e3"}},
			{RiskKeys: []string{"e1", "e2"}},
			{RiskKeys: []string{"e1"}},
		},
	}

	report := rankRisks(d, false)
	if len(report.Risks) != premortemMaxRisks {
		t.Fatalf("risks = %d, want %d", len(report.Risks), premortemMaxRisks)
	}
	// 1 周目で execution 1 枠 + hypothesis 1 枠。3 点目は候補の kind が尽きたので
	// 2 周目が件数順で埋める。
	if got := riskKeys(report.Risks); got != "e1,h1,e2" {
		t.Fatalf("risks = %q, want e1,h1,e2（execution は 1 周目で 1 枠だけ）", got)
	}
	kinds := map[string]int{}
	for _, r := range report.Risks[:2] {
		kinds[r.Kind]++
	}
	if kinds[premortemKindExecution] != 1 || kinds[premortemKindHypothesis] != 1 {
		t.Fatalf("1 周目で kind が重なっている: %+v", kinds)
	}
}

// AC-7: few で 1 点だけ採るとき、kind に関わらず件数最上位が採られる（判断系を優先しない）。
func TestRankRisks_FewTakesTopCount(t *testing.T) {
	report := rankRisks(premortemRiskFixture(), true)

	if len(report.Risks) != premortemMaxRisksFew {
		t.Fatalf("risks = %d, want %d", len(report.Risks), premortemMaxRisksFew)
	}
	if report.Risks[0].Key != "ol_slide" || report.Risks[0].Kind != premortemKindExecution {
		t.Fatalf("few の 1 点 = %q(%s), want ol_slide(execution)", report.Risks[0].Key, report.Risks[0].Kind)
	}
}

// 全候補が 1 件以下でも 0 件にせず、順位のまま見せる（focus と同じ）。
func TestRankRisks_AllSingletons(t *testing.T) {
	d := premortemDigest{
		Risks: []premortemRisk{
			{Key: "a", Kind: premortemKindExecution, Title: "A", Label: "A"},
			{Key: "b", Kind: premortemKindMatchup, Title: "B", Label: "B"},
		},
		Plays: []premortemPlay{{RiskKeys: []string{"a"}}, {RiskKeys: []string{"b"}}},
	}
	if got := riskKeys(rankRisks(d, false).Risks); got != "a,b" {
		t.Fatalf("risks = %q, want a,b", got)
	}
}

// AC-8: 判断系（hypothesis / adjust）が 1 件も無くても採用は成立する（強制しない）。
func TestHasJudgementKind(t *testing.T) {
	none := []rankedRisk{
		{premortemRisk: premortemRisk{Kind: premortemKindExecution}},
		{premortemRisk: premortemRisk{Kind: premortemKindMatchup}},
	}
	if hasJudgementKind(none) {
		t.Fatal("判断系が無いのに true")
	}
	for _, kind := range premortemJudgementKinds {
		if !hasJudgementKind([]rankedRisk{{premortemRisk: premortemRisk{Kind: kind}}}) {
			t.Fatalf("kind=%s が判断系として扱われていない", kind)
		}
	}
}

// AC-10: label は 14 文字以内に切り詰められ、空文字・空白のみなら title から作られる。
func TestNormalizeRisk_Label(t *testing.T) {
	cases := []struct {
		name string
		risk premortemRisk
		want string
	}{
		{"前後の空白を落とす", premortemRisk{Label: "  ブレイク前後の減速 ", Title: "T"}, "ブレイク前後の減速"},
		{"長すぎる label は切り詰める", premortemRisk{Label: strings.Repeat("あ", 20), Title: "T"}, strings.Repeat("あ", premortemLabelRuneLimit-1) + "…"},
		{"空なら title から作る", premortemRisk{Label: "", Title: "縦の走り込みが揃わない"}, "縦の走り込みが揃わない"},
		{"空白のみなら title から作る", premortemRisk{Label: "   ", Title: "2ndチョイスの遅れ"}, "2ndチョイスの遅れ"},
		{"title も長ければ切り詰める", premortemRisk{Label: "", Title: strings.Repeat("い", 30)}, strings.Repeat("い", premortemLabelRuneLimit-1) + "…"},
		{"どちらも空なら空", premortemRisk{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeRisk(c.risk).Label; got != c.want {
				t.Fatalf("Label = %q, want %q", got, c.want)
			}
		})
	}
}

// AC-29: enum 外の kind は「型不明」に寄せる（黙って通さない）。
func TestNormalizeRiskKind(t *testing.T) {
	for _, kind := range premortemKinds {
		if got := normalizeRiskKind(kind); got != kind {
			t.Fatalf("normalizeRiskKind(%q) = %q, want 変更なし", kind, got)
		}
	}
	if got := normalizeRiskKind("  ADJUST "); got != premortemKindAdjust {
		t.Fatalf("大文字・空白入りが正規化されていない: %q", got)
	}
	for _, bad := range []string{"", "physical", "実行"} {
		if got := normalizeRiskKind(bad); got != "" {
			t.Fatalf("normalizeRiskKind(%q) = %q, want 空文字", bad, got)
		}
	}
}

// prevent は 2 件までに切る（schema では縛れない）。
func TestNormalizeRisk_Prevent(t *testing.T) {
	risk := normalizeRisk(premortemRisk{Prevent: []string{" a ", "b", "c", "d"}})
	if len(risk.Prevent) != premortemMaxPrevent {
		t.Fatalf("Prevent = %v, want %d 件", risk.Prevent, premortemMaxPrevent)
	}
	if risk.Prevent[0] != "a" {
		t.Fatalf("前後の空白が落ちていない: %q", risk.Prevent[0])
	}
}

// AC-33 の判定ロジック。採用リスクが 2 ユニット以上にまたがるときだけ true。
func TestPremortemUnitsSpan(t *testing.T) {
	one := []rankedRisk{
		{premortemRisk: premortemRisk{Unit: "OF"}},
		{premortemRisk: premortemRisk{Unit: "OF"}},
		{premortemRisk: premortemRisk{Unit: ""}},
	}
	if premortemUnitsSpan(one) {
		t.Fatal("1 ユニットしか無いのに true")
	}
	two := append(one, rankedRisk{premortemRisk: premortemRisk{Unit: "DF"}})
	if !premortemUnitsSpan(two) {
		t.Fatal("2 ユニットにまたがるのに false")
	}
}
