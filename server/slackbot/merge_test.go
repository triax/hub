package slackbot

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

func mergeCandidates(keys ...string) []mergeCandidate {
	out := make([]mergeCandidate, 0, len(keys))
	for _, k := range keys {
		out = append(out, mergeCandidate{Key: k, Title: "見出し " + k, Label: "短い " + k})
	}
	return out
}

// AC-8 / AC-9: 応答は信用しない。入力に無い key・代表が未知のグループ・重複出現を捨てる。
func TestResolveMergeMap(t *testing.T) {
	known := mergeCandidates("a", "b", "c", "d")

	cases := []struct {
		name   string
		groups mergeGroups
		want   map[string]string
	}{
		{
			name: "同じことを言う 2 件が寄る",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: "a", MemberKeys: []string{"a", "b"}},
				{CanonicalKey: "c", MemberKeys: []string{"c"}},
			}},
			want: map[string]string{"b": "a"},
		},
		{
			name: "単独グループだけなら名寄せ無し（空 map）",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: "a", MemberKeys: []string{"a"}},
				{CanonicalKey: "b", MemberKeys: []string{"b"}},
			}},
			want: map[string]string{},
		},
		{
			name: "代表が入力に無いグループは丸ごと捨てる",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: "zzz", MemberKeys: []string{"a", "b"}},
			}},
			want: map[string]string{},
		},
		{
			name: "member が入力に無ければその 1 件だけ捨てる",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: "a", MemberKeys: []string{"a", "zzz", "b"}},
			}},
			want: map[string]string{"b": "a"},
		},
		{
			name: "同じ key が複数グループに出たら先勝ち",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: "a", MemberKeys: []string{"a", "b"}},
				{CanonicalKey: "c", MemberKeys: []string{"c", "b", "d"}},
			}},
			want: map[string]string{"b": "a", "d": "c"},
		},
		{
			name:   "グループが空でも落ちない",
			groups: mergeGroups{},
			want:   map[string]string{},
		},
		{
			name: "前後の空白は無視する",
			groups: mergeGroups{Groups: []mergeGroup{
				{CanonicalKey: " a ", MemberKeys: []string{" b "}},
			}},
			want: map[string]string{"b": "a"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveMergeMap(c.groups, known)
			if len(got) != len(c.want) {
				t.Fatalf("mapping = %v, want %v", got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Fatalf("mapping[%q] = %q, want %q（全体: %v）", k, got[k], v, got)
				}
			}
		})
	}
}

// AC-9: 対応表に無い key はそのまま通す。張り替え後の重複は落とし、入力順は保つ。
func TestCanonicalKeys(t *testing.T) {
	mapping := map[string]string{"b": "a", "d": "c"}
	got := canonicalKeys([]string{"b", "x", "a", "d", "c"}, mapping)
	if strings.Join(got, ",") != "a,x,c" {
		t.Fatalf("canonicalKeys = %v, want [a x c]", got)
	}
	if len(canonicalKeys(nil, mapping)) != 0 {
		t.Fatal("nil で落ちる")
	}
}

// keepCanonical は寄せられた側を落とし、代表だけを入力順で残す（型に依存しない）。
func TestKeepCanonical(t *testing.T) {
	type item struct{ Key string }
	items := []item{{"a"}, {"b"}, {"c"}, {"d"}}
	got := keepCanonical(items, func(i item) string { return i.Key }, map[string]string{"b": "a", "d": "c"})
	keys := []string{}
	for _, i := range got {
		keys = append(keys, i.Key)
	}
	if strings.Join(keys, ",") != "a,c" {
		t.Fatalf("keepCanonical = %v, want [a c]", keys)
	}
}

// AC-4 / AC-5: 2 段目の schema は key のグルーピングだけ。本文フィールドを持たない。
func TestMergeSchema(t *testing.T) {
	if mergeSchema["additionalProperties"] != false {
		t.Fatal("root が strict でない")
	}
	group := mergeSchema["properties"].(map[string]any)["groups"].(map[string]any)["items"].(map[string]any)
	if group["additionalProperties"] != false {
		t.Fatal("groups[] が strict でない")
	}
	props := group["properties"].(map[string]any)
	required := group["required"].([]any)
	if len(props) != 2 || len(required) != 2 {
		t.Fatalf("groups[] のフィールド = %v, want canonical_key / member_keys の 2 つだけ", props)
	}
	for _, body := range []string{"title", "label", "summary", "quote", "scenario", "signal"} {
		if _, ok := props[body]; ok {
			t.Fatalf("2 段目の schema に本文フィールド %q がある（記述をさせてはいけない）", body)
		}
	}
}

// AC-3 / AC-6: 2 段目は候補見出しだけを渡し、モデルは 1 段目と同じ。plays は渡さない。
func TestMergeCandidateKeys_Request(t *testing.T) {
	gpt := &fakeChatGPT{reply: `{"groups":[{"canonical_key":"a","member_keys":["a","b"]}]}`}
	bot := Bot{ChatGPT: gpt}

	mapping := bot.mergeCandidateKeys(t.Context(), mergeCandidates("a", "b"))
	if mapping["b"] != "a" {
		t.Fatalf("mapping = %v", mapping)
	}
	if len(gpt.requests) != 1 {
		t.Fatalf("呼び出し = %d, want 1", len(gpt.requests))
	}
	req := gpt.requests[0]
	if req.Model != chatModelFocus {
		t.Fatalf("model = %q, want %q（名寄せは意味判断なので 1 段目と同じモデル）", req.Model, chatModelFocus)
	}
	if req.Schema == nil || req.Schema.Name != mergeSchemaName {
		t.Fatalf("Structured Outputs で受けていない: %+v", req.Schema)
	}
	for _, key := range []string{"a", "b"} {
		if !strings.Contains(req.User, key) {
			t.Fatalf("候補 %q が入力に無い:\n%s", key, req.User)
		}
	}
	// plays は渡さない（件数を LLM に数えさせない）
	for _, forbidden := range []string{"plays", "theme_keys", "risk_keys"} {
		if strings.Contains(req.User, forbidden) {
			t.Fatalf("2 段目の入力に %q が混ざっている:\n%s", forbidden, req.User)
		}
	}
}

// AC-2 の裏側: 寄せる相手がいなければ 2 段目を呼ばない。
func TestMergeCandidateKeys_SkipsWhenNothingToMerge(t *testing.T) {
	gpt := &fakeChatGPT{}
	bot := Bot{ChatGPT: gpt}
	for _, candidates := range [][]mergeCandidate{nil, mergeCandidates("a")} {
		if m := bot.mergeCandidateKeys(t.Context(), candidates); len(m) != 0 {
			t.Fatalf("mapping = %v, want 空", m)
		}
	}
	if len(gpt.requests) != 0 {
		t.Fatalf("呼び出し = %d, want 0", len(gpt.requests))
	}
}

// AC-8: 2 段目が壊れても全体を落とさない。名寄せせず続行する。
func TestMergeCandidateKeys_FailsSoft(t *testing.T) {
	cases := []struct {
		name string
		gpt  *fakeChatGPT
	}{
		{"parse できない", &fakeChatGPT{reply: "これは JSON ではありません"}},
		{"呼び出しが失敗する", &fakeChatGPT{err: errors.New("boom")}},
		{"key を新造してくる", &fakeChatGPT{reply: `{"groups":[{"canonical_key":"新しいkey","member_keys":["a","b"]}]}`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bot := Bot{ChatGPT: c.gpt}
			if m := bot.mergeCandidateKeys(t.Context(), mergeCandidates("a", "b")); len(m) != 0 {
				t.Fatalf("mapping = %v, want 空（名寄せせず続行）", m)
			}
		})
	}
}

// ---- 分割 → 2 段目 reduce の結合検証 ------------------------------------------

// longThreads は budget を超えさせるための、本文の長い返信付き投稿を n 件作る。
func longThreads(n, runes int) []playThread {
	body := strings.Repeat("あ", runes)
	threads := make([]playThread, 0, n)
	for i := 0; i < n; i++ {
		ts := fmt.Sprintf("%d.000000", 100+i)
		threads = append(threads, playThread{
			Parent:  parentMsg(ts, body, 1),
			Replies: []slack.Message{replyMsg(ts+"1", "U1", "反省")},
		})
	}
	return threads
}

// AC-2: 1 塊のときは 2 段目を呼ばない（呼び出しは 1 回のまま）。
func TestFocus_NoMergeWhenSingleChunk(t *testing.T) {
	gpt := &fakeChatGPT{reply: `{"themes":[{"key":"t1","title":"T","label":"L","summary":"S","do":[],"dont":[],"positions":[],"quote":""}],"plays":[{"headline":"","name":"P","theme_keys":["t1"],"positions":[],"issue":"i"}]}`}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}

	if _, err := bot.summarize(t.Context(), testJob(), playThreads(6), nil); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if len(gpt.requests) != 1 {
		t.Fatalf("呼び出し = %d, want 1（分割していないので 2 段目は不要）", len(gpt.requests))
	}
}

// AC-3 / AC-5 / AC-6 / AC-7: 分割された入力で、塊ごとに別 key で出た同じ課題が
// 名寄せされ、件数が合算され、採用が 1〜3 点に収まる。2 段目は 1 回だけ。
func TestFocus_MergesSplitDuplicates(t *testing.T) {
	chunk := func(key string) string {
		return fmt.Sprintf(`{"themes":[{"key":%q,"title":"ブレイク前に減速してタイミングがずれる","label":"ブレイク前の減速","summary":"S","do":["やる"],"dont":[],"positions":["WR"],"quote":"引用"}],`+
			`"plays":[{"headline":"","name":"P1","theme_keys":[%q],"positions":["WR"],"issue":"i"},`+
			`{"headline":"","name":"P2","theme_keys":[%q],"positions":["WR"],"issue":"i"},`+
			`{"headline":"","name":"P3","theme_keys":[%q],"positions":["WR"],"issue":"i"}]}`, key, key, key, key)
	}
	gpt := &fakeChatGPT{replies: []string{
		chunk("t1"), chunk("t2"),
		`{"groups":[{"canonical_key":"t1","member_keys":["t1","t2"]}]}`,
	}}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}

	// 6 スレッド × 35,000 rune = 210,000 > budget(120,000) → 2 塊に割れる
	threads := longThreads(6, 35000)
	summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if len(gpt.requests) != 3 {
		t.Fatalf("呼び出し = %d, want 3（塊 2 + 2 段目 1）", len(gpt.requests))
	}
	if gpt.requests[2].Schema == nil || gpt.requests[2].Schema.Name != mergeSchemaName {
		t.Fatalf("3 回目が 2 段目でない: %+v", gpt.requests[2].Schema)
	}
	if summary.Report == nil {
		t.Fatal("構造化に失敗している")
	}
	if n := len(summary.Report.Focus); n != 1 {
		t.Fatalf("focus = %d 点, want 1（名寄せで 1 テーマに寄る）", n)
	}
	if n := len(summary.Report.Focus); n > focusMaxThemes {
		t.Fatalf("focus = %d 点, want <= %d", n, focusMaxThemes)
	}
	got := summary.Report.Focus[0]
	if got.Key != "t1" {
		t.Fatalf("代表 key = %q, want t1", got.Key)
	}
	if got.Count != 6 {
		t.Fatalf("件数 = %d, want 6（両方の塊の plays が合算される）", got.Count)
	}
	// AC-6: 本文は 1 段目の代表 key の出力そのまま（2 段目に書き直させていない）
	if got.Title != "ブレイク前に減速してタイミングがずれる" || got.Quote != "引用" {
		t.Fatalf("本文が書き換わっている: title=%q quote=%q", got.Title, got.Quote)
	}
}

// AC-3 / AC-8: 2 段目が失敗しても、名寄せせず連結した結果で続行する。
func TestFocus_MergeFailureFallsBackToConcat(t *testing.T) {
	chunk := func(key string) string {
		return fmt.Sprintf(`{"themes":[{"key":%q,"title":"T%s","label":"L%s","summary":"S","do":[],"dont":[],"positions":[],"quote":""}],`+
			`"plays":[{"headline":"","name":"P1","theme_keys":[%q],"positions":[],"issue":"i"},`+
			`{"headline":"","name":"P2","theme_keys":[%q],"positions":[],"issue":"i"}]}`, key, key, key, key, key)
	}
	gpt := &fakeChatGPT{replies: []string{chunk("t1"), chunk("t2"), "壊れた応答"}}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}

	summary, err := bot.summarize(t.Context(), testJob(), longThreads(6, 35000), nil)
	if err != nil {
		t.Fatalf("2 段目の失敗で全体が落ちている: %v", err)
	}
	if summary.Report == nil {
		t.Fatal("構造化に失敗している")
	}
	if n := len(summary.Report.Focus); n != 2 {
		t.Fatalf("focus = %d 点, want 2（名寄せせず連結したまま）", n)
	}
}

// AC-7: premortem では、名寄せ後に kind の重複回避が全体に対して 1 回だけ効く。
func TestPremortem_MergeMakesKindDiversityGlobal(t *testing.T) {
	risk := func(key, kind, title string) string {
		return fmt.Sprintf(`{"key":%q,"kind":%q,"title":%q,"label":%q,"scenario":"S","phase":"3rd&long","unit":"OF","signal":"sig","prevent":["p"],"positions":["OL"],"quote":"q"}`,
			key, kind, title, title)
	}
	play := func(keys string) string {
		return fmt.Sprintf(`{"name":"P","risk_keys":[%s]}`, keys)
	}
	chunk1 := fmt.Sprintf(`{"risks":[%s,%s],"plays":[%s,%s,%s]}`,
		risk("e1", premortemKindExecution, "スライドが遅れる"),
		risk("h1", premortemKindHypothesis, "相手の想定違い"),
		play(`"e1","h1"`), play(`"e1"`), play(`"e1","h1"`))
	chunk2 := fmt.Sprintf(`{"risks":[%s],"plays":[%s,%s,%s]}`,
		risk("e2", premortemKindExecution, "スライドの合図が遅い"),
		play(`"e2"`), play(`"e2"`), play(`"e2"`))

	gpt := &fakeChatGPT{replies: []string{
		chunk1, chunk2,
		`{"groups":[{"canonical_key":"e1","member_keys":["e1","e2"]},{"canonical_key":"h1","member_keys":["h1"]}]}`,
	}}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}

	job := premortemJob{Channel: "C1", Sources: []string{"C1"}, MentionTS: testMentionTS}
	summary, err := bot.summarizePremortem(t.Context(), job, bot.premortemSinkFor(job), longThreads(6, 35000), nil)
	if err != nil {
		t.Fatalf("summarizePremortem: %v", err)
	}
	if len(gpt.requests) != 3 {
		t.Fatalf("呼び出し = %d, want 3（塊 2 + 2 段目 1）", len(gpt.requests))
	}
	if summary.Report == nil {
		t.Fatal("構造化に失敗している")
	}
	kinds := map[string]int{}
	for _, r := range summary.Report.Risks {
		kinds[r.Kind]++
	}
	for kind, n := range kinds {
		if n > 1 {
			t.Fatalf("kind %q が %d 件採用されている（名寄せ後は全体で 1 回だけ効くはず）: %+v", kind, n, kinds)
		}
	}
	// e2 が e1 に寄るので、採用は e1(execution) と h1(hypothesis) の 2 点
	if got := len(summary.Report.Risks); got != 2 {
		t.Fatalf("採用 = %d 点, want 2", got)
	}
	if summary.Report.Risks[0].Key != "e1" || summary.Report.Risks[0].Count != 6 {
		t.Fatalf("e1 = %+v, want key=e1 count=6（e2 の plays が合算される）",
			summary.Report.Risks[0])
	}
}
