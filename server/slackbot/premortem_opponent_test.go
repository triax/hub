package slackbot

import (
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
)

// premortemOpponentDigest は相手軸つきの候補。opponent_quote は corpus に載せる前提の原文。
func premortemOpponentDigest() premortemDigest {
	d := premortemRiskFixture()
	d.Risks[0].Opponent = "相手は 3rd&long でスクリーンとドローの比率が高い"
	d.Risks[0].OpponentQuote = "相手のスクリーン多い"
	d.Risks[1].Opponent = "相手はゴール前でタイト 2 枚からのパワーが中心"
	d.Risks[1].OpponentQuote = "ゴール前はタイト2枚"
	return d
}

const premortemOpponentCorpus = "[投稿] 前節フィルム\n  - コーチ: 相手のスクリーン多いので注意\n" +
	"  - コーチ: ゴール前はタイト2枚からのパワー\n"

// ------------------------------------------------------------------ 幻覚ガード ---

// AC-7 / AC-8 / AC-9 / AC-10: opponent は opponent_quote の逐語照合で裏を取る。
// 相手名では照合しない（カレンダーの表記と Slack の呼び方が一致しないため）。
func TestDropUnbackedOpponents(t *testing.T) {
	for _, tc := range []struct {
		name     string
		quote    string
		corpus   string
		wantKept bool
	}{
		{"引用が逐語で見つかれば残す", "相手のスクリーン多い", premortemOpponentCorpus, true},
		{"引用が見つからなければ落とす", "相手はラン主体のチーム", premortemOpponentCorpus, false},
		{"引用が空なら落とす（根拠なしの相手評を通さない）", "", premortemOpponentCorpus, false},
		{"空白と改行の差は吸収する", "相手の スクリーン\n多い", premortemOpponentCorpus, true},
		{"似ているだけの文字列は通さない", "相手のスクリーンが多い", premortemOpponentCorpus, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			risks := []premortemRisk{{
				Key: "k", Opponent: "相手はスクリーンが多い", OpponentQuote: tc.quote,
			}}
			got := dropUnbackedOpponents(risks, tc.corpus)
			if kept := got[0].Opponent != ""; kept != tc.wantKept {
				t.Fatalf("Opponent の残存 = %v, want %v（Opponent=%q Quote=%q）",
					kept, tc.wantKept, got[0].Opponent, got[0].OpponentQuote)
			}
			// 落とすときは引用も道連れにする（片方だけ残ると後段が「根拠あり」と誤認する）
			if !tc.wantKept && got[0].OpponentQuote != "" {
				t.Fatalf("Opponent を落としたのに OpponentQuote が残っている: %q", got[0].OpponentQuote)
			}
		})
	}
}

// AC-11: 同じ相手の前提が 2 枚のカードに出ると、新しい情報が増えたように見えて増えていない。
// 採用が確定した後に、順位が上のものにだけ残す。
func TestDedupeOpponents(t *testing.T) {
	risks := []rankedRisk{
		{premortemRisk: premortemRisk{Key: "a", Opponent: "相手はランが中心", OpponentQuote: "q1"}},
		{premortemRisk: premortemRisk{Key: "b", Opponent: "相手はランが中心", OpponentQuote: "q2"}},
		{premortemRisk: premortemRisk{Key: "c", Opponent: "相手のセカンダリは 3 人", OpponentQuote: "q3"}},
	}
	got := dedupeOpponents(risks)
	if got[0].Opponent == "" {
		t.Fatal("順位が上の 1 件目が落ちている")
	}
	if got[1].Opponent != "" || got[1].OpponentQuote != "" {
		t.Fatalf("重複した 2 件目が残っている: %q / %q", got[1].Opponent, got[1].OpponentQuote)
	}
	if got[2].Opponent == "" {
		t.Fatal("重複していない 3 件目まで落ちている")
	}
}

// ------------------------------------------------------------------ 描画（案 E） ---

// AC-12: 相手の前提は scenario の頭に連結して 1 段落にする。ラベルも専用ブロックも付けない。
// 枠を作ると、材料が「あるが薄い」ときに空席が見えてしまう（#698 決定 2）。
func TestPremortemRiskBody_ConcatenatesOpponent(t *testing.T) {
	r := rankedRisk{premortemRisk: premortemRisk{
		Opponent: "相手は 3rd&long でスクリーンが多い",
		Scenario: "対してこちらは LB が前を見られていない。",
	}}
	got := premortemRiskBody(r)
	want := "相手は 3rd&long でスクリーンが多い。対してこちらは LB が前を見られていない。"
	if got != want {
		t.Fatalf("本文 =\n %q\nwant\n %q", got, want)
	}
	if strings.Contains(got, "相手:") {
		t.Fatalf("ラベルが出ている（案 B を採らなかったのに）: %q", got)
	}
}

// 句点で終わっている opponent に句点を足さない（「。。」にしない）。
func TestPremortemRiskBody_DoesNotDoublePunctuate(t *testing.T) {
	r := rankedRisk{premortemRisk: premortemRisk{
		Opponent: "相手はランが中心。", Scenario: "こちらはセットが遅れる。",
	}}
	if got := premortemRiskBody(r); strings.Contains(got, "。。") {
		t.Fatalf("句点が重なっている: %q", got)
	}
}

// AC-13: opponent と scenario は独立に切り詰める（まとめて切ると文の途中で切れる）。
func TestPremortemRiskBody_TruncatesIndependently(t *testing.T) {
	r := rankedRisk{premortemRisk: premortemRisk{
		Opponent: strings.Repeat("相", premortemOpponentRuneLimit+50),
		Scenario: strings.Repeat("我", premortemScenarioRuneLimit+50),
	}}
	got := []rune(premortemRiskBody(r))
	// 切り詰めた側にはそれぞれ … が入り、句点が 1 つ足される
	if want := premortemOpponentRuneLimit + 1 + premortemScenarioRuneLimit; len(got) != want {
		t.Fatalf("本文の長さ = %d, want %d（opponent %d + 句点 1 + scenario %d）",
			len(got), want, premortemOpponentRuneLimit, premortemScenarioRuneLimit)
	}
}

// AC-14 / AC-15: 材料が無いときは誘う。欠落を詫びない（#698 決定 4）。
func TestPremortemGuide_InvitesWhenNoOpponent(t *testing.T) {
	job := premortemTestJob()

	without := premortemGuide(job, false)
	if !strings.Contains(without, "相手の資料をこのチャンネルに貼ると") {
		t.Fatalf("材料が無いのに誘いが無い: %q", without)
	}
	for _, ng := range []string{"見当たりません", "ありません", "不足", "できませんでした"} {
		if strings.Contains(without, ng) {
			t.Fatalf("欠落を詫びる文言 %q が入っている（相手情報は上乗せであって土台ではない）: %q", ng, without)
		}
	}

	with := premortemGuide(job, true)
	if strings.Contains(with, "相手の資料をこのチャンネルに貼ると") {
		t.Fatalf("材料があるのに誘いが出ている: %q", with)
	}
	// #697 の射程申告は両方で残る
	for name, got := range map[string]string{"あり": with, "なし": without} {
		if !strings.HasPrefix(got, "この premortem は <#C1> に書かれたことだけを材料にしています。") {
			t.Fatalf("材料%s: 射程の申告が消えている: %q", name, got)
		}
	}
}

func TestHasOpponentBasis(t *testing.T) {
	none := []rankedRisk{{premortemRisk: premortemRisk{Key: "a"}}, {premortemRisk: premortemRisk{Key: "b"}}}
	if hasOpponentBasis(none) {
		t.Fatal("全件空なのに true")
	}
	some := append([]rankedRisk{}, none...)
	some[1].Opponent = "相手はランが中心"
	if !hasOpponentBasis(some) {
		t.Fatal("1 件あるのに false")
	}
}

// ------------------------------------------------------------------ 大原則 ---

// AC-18 / AC-19: 相手情報がゼロでも premortem は成立する。
// これが崩れると、スカウティングを貼っていないチャンネルで質が落ちる（#698 大原則）。
func TestPremortem_StandsWithoutOpponent(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	job := premortemTestJob()

	bare := rankRisks(premortemRiskFixture(), false) // opponent 一切なし
	withOpp := rankRisks(premortemOpponentDigest(), false)

	bareBlocks := premortemDigestBlocks(job, playThreads(8), "9/21(日) vs A", now, bare)
	oppBlocks := premortemDigestBlocks(job, playThreads(8), "9/21(日) vs A", now, withOpp)

	// AC-19: ブロックの型の並びが opponent の有無で変わらない
	bareTypes := strings.Join(blockTypes(bareBlocks), ",")
	oppTypes := strings.Join(blockTypes(oppBlocks), ",")
	if bareTypes != oppTypes {
		t.Fatalf("opponent の有無でブロック構成が変わった:\n なし=%s\n あり=%s", bareTypes, oppTypes)
	}
	if len(bareBlocks) != premortemDigestFixedBlocks+premortemMaxRisks*premortemBlocksPerRisk {
		t.Fatalf("block 数 = %d, want %d", len(bareBlocks),
			premortemDigestFixedBlocks+premortemMaxRisks*premortemBlocksPerRisk)
	}

	// AC-18: 本文が scenario だけで構成され、先頭に余分な区切りが残らない
	for i, r := range bare.Risks {
		got := premortemRiskBody(r)
		if got != r.Scenario {
			t.Fatalf("risks[%d]: opponent が無いのに本文が scenario と違う:\n got=%q\nwant=%q", i, got, r.Scenario)
		}
	}
	body := blocksJSON(t, bareBlocks)
	if strings.Contains(body, "相手:") {
		t.Fatalf("opponent が無いのに相手のラベルが出ている: %s", body)
	}
}

// AC-20: 相手が引けず opponent も全件空でも、出力の骨格が揃う。
func TestPremortem_NoGameNoOpponent_StillComplete(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	job := premortemTestJob()
	job.Oldest = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC).Unix()

	blocks := premortemDigestBlocks(job, playThreads(8), "", now, rankRisks(premortemRiskFixture(), false))
	body := blocksJSON(t, blocks)
	for _, want := range []string{
		"の premortem",    // 見出し（期間ラベルに落ちる）
		"8/30〜9/11",      // メタ行の期間
		"<#C1>",          // メタ行の射程
		"3rd&long の被サック", // 目次
		"⚠️ 試合中に見るもの",    // 兆候の集約
		"に書かれたことだけを材料にしています", // 末尾の案内
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("相手も材料も無い出力に %q が無い（骨格が欠けている）: %s", want, body)
		}
	}
}

// ------------------------------------------------------------------ プロンプト ---

// AC-1 / AC-2 / AC-4: 相手とチャンネルのスコープが、要約の入力に載る。
func TestPremortemPromptPreamble(t *testing.T) {
	got := premortemPromptPreamble(premortemPromptContext{
		Game: "9/21(日) vs シルバースター", Scope: "defense", Extras: []string{"#scout"},
	})
	for _, want := range []string{
		"9/21(日) vs シルバースター", "#defense", "#scout", "マッチアップ",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("preamble に %q が無い: %q", want, got)
		}
	}
	// 主スコープと参考は区別する（#698 決定 7）
	if strings.Index(got, "#defense") > strings.Index(got, "#scout") {
		t.Fatalf("参考チャンネルが主スコープより先に出ている: %q", got)
	}
}

// AC-3 / AC-5: 試合もチャンネル名も引けないとき、preamble は空になり、
// プロンプトは相手軸の無い形（#683 と同じ）に戻る。
func TestPremortemPromptPreamble_EmptyWhenNothingResolved(t *testing.T) {
	if got := premortemPromptPreamble(premortemPromptContext{}); got != "" {
		t.Fatalf("何も引けないのに preamble が出ている: %q", got)
	}
	bare := premortemSystemPrompt(false, premortemPromptContext{})
	if !strings.HasPrefix(bare, "あなたはアメリカンフットボールチームのコーチ補佐です。") {
		t.Fatalf("相手軸なしのプロンプトが #683 の形と違う:\n%s", bare[:120])
	}
	withCtx := premortemSystemPrompt(false, premortemPromptContext{Game: "9/21(日) vs A", Scope: "defense"})
	if !strings.Contains(withCtx, "9/21(日) vs A") || !strings.Contains(withCtx, "#defense") {
		t.Fatalf("文脈がプロンプトに載っていない:\n%s", withCtx[:200])
	}
}

// AC-4 / AC-5: チャンネル名の解決。読めなければ空文字を返し、premortem は止めない。
func TestPremortemPromptContextFor_ChannelNames(t *testing.T) {
	api := newFakeSlackAPI()
	api.channelInfoByID = map[string]*slack.Channel{
		"C1": channelNamed("C1", "defense"),
		"C2": channelNamed("C2", "scout"),
	}
	bot := Bot{SlackAPI: api}
	job := premortemJob{Channel: "C1", Sources: []string{"C1", "C2", "C3"}}

	pc := bot.premortemPromptContextFor(t.Context(), job, time.Now())
	if pc.Scope != "defense" {
		t.Fatalf("主スコープ = %q, want defense", pc.Scope)
	}
	// C3 は引けない。落とすだけで止めない
	if len(pc.Extras) != 1 || pc.Extras[0] != "#scout" {
		t.Fatalf("参考チャンネル = %v, want [#scout]（読めない C3 は落とす）", pc.Extras)
	}
}

// AC-5: conversations.info が全滅しても premortem は完走する。
func TestPremortemPromptContextFor_SurvivesInfoFailure(t *testing.T) {
	bot := Bot{SlackAPI: newFakeSlackAPI()} // channelInfo 未設定 → 常にエラー
	job := premortemJob{Channel: "C1", Sources: []string{"C1"}}

	pc := bot.premortemPromptContextFor(t.Context(), job, time.Now())
	if pc.Scope != "" || len(pc.Extras) != 0 {
		t.Fatalf("読めないのに名前が入っている: %+v", pc)
	}
	if got := premortemSystemPrompt(false, pc); !strings.HasPrefix(got, "あなたはアメリカン") {
		t.Fatalf("プロンプトが壊れている:\n%s", got[:120])
	}
}

// AC-6: schema に opponent / opponent_quote が required で入る（strict なので空文字許容）。
func TestPremortemReportSchema_HasOpponentFields(t *testing.T) {
	risks, ok := premortemReportSchema["properties"].(map[string]any)["risks"].(map[string]any)
	if !ok {
		t.Fatal("schema の risks を読めない")
	}
	item := risks["items"].(map[string]any)
	props := item["properties"].(map[string]any)
	required, _ := item["required"].([]any)

	for _, field := range []string{"opponent", "opponent_quote"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("schema に %s が無い", field)
		}
		found := false
		for _, r := range required {
			if r == field {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s が required に入っていない（strict では全 property が required）", field)
		}
	}
}

func channelNamed(id, name string) *slack.Channel {
	ch := &slack.Channel{}
	ch.ID = id
	ch.Name = name
	return ch
}
