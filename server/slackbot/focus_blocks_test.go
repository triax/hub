package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/slack-go/slack"
)

// digestJSON は LLM が返す構造化出力。テーマ 4 件・プレー 6 件で、
// theme_keys の分布は timing=3 / vertical=2 / call=2 / stance=1。
// stance は件数 1 なので focus には採られない（単発は詳細側の材料）。
// call だけ label を持たせず、title からのフォールバック（#681）を踏ませる。
//
// do / dont は正規化と描画の分岐を 1 つの入力で踏めるように散らしてある:
// timing は do / dont とも 3 件（空文字・重複入り。2 件に切り詰まる）、
// vertical は dont が空（「やめる」を省く）、call は do が空（「やる」を省く）。
const digestJSON = `{
  "themes": [
    {"key":"timing","title":"QB↔WR のタイミング","label":"タイミング","summary":"スナップ前に MOFO/MOFC を決めておらず、投げ先の判断がブレイク後になっている。","positions":["QB","WR"],
     "quote":"Xがピタッと止まれてないのと、QBが待ちすぎ",
     "do":["ブレイク 3 歩目でボールを離す","  ","ブレイク 3 歩目でボールを離す","スナップ前に SF の目線で MOFO/MOFC を決める","フラットは最後に読む"],
     "dont":["フラットを第一選択にして待つ","","投げ急いでリズムを崩す","ブレイク後に投げ先を決める"]},
    {"key":"vertical","title":"縦の走り込み","label":"縦の走り込み","summary":"3 歩目で減速して縦が死に、SF を釣れていない。","positions":["WR"],
     "quote":"3歩目で減速して縦が死んでいる",
     "do":["奥まで駆け抜けてから切る","縦の 5 歩目まで減速しない"],"dont":[]},
    {"key":"call","title":"セット前のコール","summary":"SF の位置を声に出しておらず、コールが後ろまで届いていない。","positions":[],
     "quote":"コールが聞こえなくて合わせられなかった",
     "do":[],"dont":["黙ってセットする"]},
    {"key":"stance","title":"スタンスの幅","label":"スタンスの幅","summary":"スタンスが狭く、内側を割られている。","positions":["OL"],
     "quote":"スタンスが狭くて割られた",
     "do":["肩幅より広く構える"],"dont":["狭いスタンスで構える"]}
  ],
  "plays": [
    {"headline":"GL Drive1","name":"プレーA","theme_keys":["timing","vertical"],"positions":["QB","WR"],"issue":"リリースが 1 テンポ遅い"},
    {"headline":"GL Drive1","name":"プレーB","theme_keys":["timing"],"positions":["QB"],"issue":"フラットを第一選択にした"},
    {"headline":"GL Drive1","name":"プレーC","theme_keys":["vertical","call"],"positions":["WR"],"issue":"3 歩目で減速した"},
    {"headline":"skel","name":"プレーD","theme_keys":["timing"],"positions":["QB","WR"],"issue":"ブレイク前にボールが出た"},
    {"headline":"skel","name":"プレーE","theme_keys":["call"],"positions":[],"issue":"コールが通らなかった"},
    {"headline":"skel","name":"プレーF","theme_keys":["stance"],"positions":["OL"],"issue":"スタンスを割られた"}
  ]
}`

// playThreads は返信の付いたプレー投稿を n 件組む。focusFewTargetsThreshold 以上
// 渡すと few 扱いを外れ、focus が focusMaxThemes まで採られる。
func playThreads(n int) []playThread {
	threads := make([]playThread, 0, n)
	for i := 0; i < n; i++ {
		ts := fmt.Sprintf("%03d.000000", 100+i)
		threads = append(threads, playThread{
			Parent:  parentMsg(ts, "プレー", 1),
			Replies: []slack.Message{replyMsg(ts+"1", "U1", "反省")},
		})
	}
	return threads
}

// sampleDigest は digestJSON を LLM 越しに通さず直接 focusDigest に戻す
// （few の判定に引きずられずに描画だけを見たいとき用）。
func sampleDigest(t *testing.T) focusDigest {
	t.Helper()
	d := focusDigest{}
	if err := json.Unmarshal([]byte(digestJSON), &d); err != nil {
		t.Fatalf("digestJSON: %v", err)
	}
	return d
}

func sampleReport(t *testing.T) focusReport {
	t.Helper()
	gpt := &fakeChatGPT{reply: digestJSON}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
	summary, err := bot.summarize(t.Context(), testJob(), playThreads(focusFewTargetsThreshold), nil)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.Report == nil {
		t.Fatal("report が decode されていない")
	}
	return *summary.Report
}

// blockTypes は digestBlocks の戻り値の並びを type だけの列にする
// （sentMessage.BlockTypes の、Slack に投げずに使える版）。
func blockTypes(blocks []slack.Block) []string {
	types := make([]string, 0, len(blocks))
	for _, b := range blocks {
		types = append(types, string(b.BlockType()))
	}
	return types
}

// marshalBlocks は digestBlocks の戻り値を、Slack へ送られるのと同じ JSON の
// 素の map に戻す（sentMessage.Blocks の、投稿を挟まない版）。
func marshalBlocks(t *testing.T, blocks []slack.Block) []map[string]any {
	t.Helper()
	raw, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("blocks の marshal: %v", err)
	}
	out := []map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("blocks の unmarshal: %v", err)
	}
	return out
}

// blockText は block の map から text を掘り出す（header / section / context 用）。
func blockText(block map[string]any) string {
	if text, ok := block["text"].(map[string]any); ok {
		s, _ := text["text"].(string)
		return s
	}
	if elements, ok := block["elements"].([]any); ok && len(elements) > 0 {
		if first, ok := elements[0].(map[string]any); ok {
			if s, ok := first["text"].(string); ok {
				return s
			}
		}
	}
	return ""
}

// richTextElements は rich_text ブロック直下の要素（section / list）を取り出す。
func richTextElements(t *testing.T, block map[string]any) []map[string]any {
	t.Helper()
	elements := []map[string]any{}
	for _, e := range block["elements"].([]any) {
		elements = append(elements, e.(map[string]any))
	}
	return elements
}

// richTextElementTypes は要素の type 列（`rich_text_section,rich_text_list,…`）。
func richTextElementTypes(elements []map[string]any) []string {
	types := []string{}
	for _, e := range elements {
		s, _ := e["type"].(string)
		types = append(types, s)
	}
	return types
}

// sectionText は rich_text_section の text を連ねた 1 本の文字列。
// boldFirst は先頭要素が太字かどうか（見出し・ラベルの検査用）。
func sectionText(section map[string]any) string {
	buf := &strings.Builder{}
	for _, e := range section["elements"].([]any) {
		s, _ := e.(map[string]any)["text"].(string)
		buf.WriteString(s)
	}
	return buf.String()
}

func boldFirst(section map[string]any) bool {
	first, ok := section["elements"].([]any)[0].(map[string]any)
	if !ok {
		return false
	}
	style, ok := first["style"].(map[string]any)
	if !ok {
		return false
	}
	bold, _ := style["bold"].(bool)
	return bold
}

// richTextItemText は rich_text_list の i 番目の項目を、要素の text を連ねた 1 本の
// 文字列にする（bold / plain の別は問わず「何が書かれているか」だけを見る）。
func richTextItemText(t *testing.T, list map[string]any, i int) string {
	t.Helper()
	return sectionText(list["elements"].([]any)[i].(map[string]any))
}

// #658 AC-1: JSON を focusDigest に decode し、rankThemes が件数順の focus を組む。
// count は theme_keys の実数で、LLM の自己申告ではない。
func TestSummarize_DecodesReport(t *testing.T) {
	report := sampleReport(t)

	if len(report.Focus) != 3 {
		t.Fatalf("focus = %d 件, want 3（件数 1 の stance は除外）", len(report.Focus))
	}
	wantTitles := []string{"QB↔WR のタイミング", "縦の走り込み", "セット前のコール"}
	wantCounts := []int{3, 2, 2}
	for i, want := range wantTitles {
		if got := report.Focus[i].Title; got != want {
			t.Fatalf("focus[%d].Title = %q, want %q（件数順に並んでいない）", i, got, want)
		}
		if got := report.Focus[i].Count; got != wantCounts[i] {
			t.Fatalf("focus[%d].Count = %d, want %d（theme_keys の実数）", i, got, wantCounts[i])
		}
	}
	if got := strings.Join(report.Focus[0].Plays, ","); got != "プレーA,プレーB,プレーD" {
		t.Fatalf("focus[0].Plays = %q, want 代表プレー（入力順）", got)
	}
	if got := strings.Join(report.Focus[0].Positions, ","); got != "QB,WR" {
		t.Fatalf("focus[0].Positions = %q", got)
	}
	if report.Focus[0].Quote == "" || report.Focus[0].Summary == "" {
		t.Fatalf("focus[0] に quote / summary が乗っていない: %+v", report.Focus[0])
	}
	// #661 AC-2: do / dont は空文字と重複を落として各 2 件に切り詰まる。
	if got := strings.Join(report.Focus[0].Do, "|"); got != "ブレイク 3 歩目でボールを離す|スナップ前に SF の目線で MOFO/MOFC を決める" {
		t.Fatalf("focus[0].Do = %q, want 空文字・重複を除いた先頭 2 件", got)
	}
	if got := strings.Join(report.Focus[0].Dont, "|"); got != "フラットを第一選択にして待つ|投げ急いでリズムを崩す" {
		t.Fatalf("focus[0].Dont = %q, want 空文字を除いた先頭 2 件", got)
	}
	// do が 0 件のテーマ（call）も focus から外さない。
	if len(report.Focus[2].Do) != 0 || len(report.Focus[2].Dont) != 1 {
		t.Fatalf("focus[2] = %+v, want do 0 件・dont 1 件のまま focus に残る", report.Focus[2])
	}
	if len(report.Plays) != 6 || report.Plays[0].Name != "プレーA" {
		t.Fatalf("plays = %+v, want 入力順の 6 件", report.Plays)
	}
	// 集計は #659 のチャートが読む。件数 1 のテーマも Stats には残る。
	if len(report.Stats.Themes) != 4 || report.Stats.Themes[0].Count != 3 {
		t.Fatalf("Stats.Themes = %+v, want 4 件（件数降順）", report.Stats.Themes)
	}
	// #681 AC-2/AC-3: label は目次と pie 凡例に出す短い名前。無ければ title に倒す。
	wantLabels := []string{"タイミング", "縦の走り込み", "セット前のコール"}
	for i, want := range wantLabels {
		if got := report.Focus[i].Label; got != want {
			t.Fatalf("focus[%d].Label = %q, want %q", i, got, want)
		}
	}
}

// AC-6: structured output が読めなかったときは、エラーにせず平文フォールバックへ
// 倒れ、その旨が log に出る（安全弁の維持）。
func TestSummarize_BrokenJSONFallback(t *testing.T) {
	threads := []playThread{{Parent: parentMsg("100.000000", "プレーA", 1),
		Replies: []slack.Message{replyMsg("101.000000", "U1", "反省1")}}}

	t.Run("壊れた JSON は平文フォールバック", func(t *testing.T) {
		buf := &bytes.Buffer{}
		restore := log.Writer()
		log.SetOutput(buf)
		defer log.SetOutput(restore)

		gpt := &fakeChatGPT{reply: "*プレーA* 縦の走り込みを揃える。"}
		bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
		summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
		if err != nil {
			t.Fatalf("壊れた JSON が error になっている: %v", err)
		}
		if summary.Report != nil {
			t.Fatalf("壊れた JSON なのに report が返っている: %+v", summary.Report)
		}
		if summary.Text != "*プレーA* 縦の走り込みを揃える。" {
			t.Fatalf("平文フォールバックの本文が失われている: %q", summary.Text)
		}
		if !strings.Contains(buf.String(), "structured output を受け取れませんでした") {
			t.Fatalf("フォールバックが log に残っていない: %q", buf.String())
		}
	})

	t.Run("focus 0 件も平文フォールバック", func(t *testing.T) {
		gpt := &fakeChatGPT{reply: `{"themes":[],"plays":[]}`}
		bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
		summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
		if err != nil {
			t.Fatalf("summarize: %v", err)
		}
		if summary.Report != nil {
			t.Fatal("focus 0 件で report を返している（空の rich_text_list は invalid_blocks になる）")
		}
	})
}

// #661 AC-3 / #681: 1 通目は header → context → rich_text（目次）→ テーマごとの
// カード → divider → context。チャンネルにも出す（reply_broadcast=true）。
// focusFixture は返信付きプレーが 2 件（= few）なので、#681 AC-20 と同じく focus は 1 点。
func TestFocus_DigestBlocks_FirstMessage(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: digestJSON}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runFocus(t.Context(), testJob()); err != nil {
		t.Fatalf("runFocus: %v", err)
	}
	if len(api.posted) < 3 {
		t.Fatalf("posted = %d, want 受付 + digest + 詳細", len(api.posted))
	}

	digest := api.posted[1] // posted[0] は受付メッセージ
	want := "header,context,rich_text,divider,section,rich_text,context,divider,context"
	if got := strings.Join(digest.BlockTypes(), ","); got != want {
		t.Fatalf("1 通目の blocks = %q, want %q", got, want)
	}
	if got := digest.Broadcast(); got != "true" {
		t.Fatalf("1 通目の reply_broadcast = %q, want true", got)
	}
	// #681 AC-14: 通知プレビューは長い title ではなく短い label を並べる。
	if got := digest.Text(); !strings.Contains(got, "1. タイミング") || strings.Contains(got, "QB↔WR のタイミング") {
		t.Fatalf("text フォールバック = %q, want label 並び", got)
	}

	blocks := digest.Blocks()
	if n := utf8.RuneCountInString(blockText(blocks[0])); n > focusHeaderRuneLimit {
		t.Fatalf("header が %d 文字（上限 %d）", n, focusHeaderRuneLimit)
	}
	if got := blockText(blocks[1]); !strings.Contains(got, "2 プレー / 4 件の反省から") {
		t.Fatalf("context の件数メタが期待どおりでない: %q", got)
	}
	// #681 AC-20: 目次もカードも 1 件だけ。
	toc := richTextElements(t, blocks[2])[0]
	if n := len(toc["elements"].([]any)); n != 1 {
		t.Fatalf("目次 = %d 件, want 1（few は 1 点に絞る）", n)
	}

	// 続きは同じスレッドに broadcast 無しで出る。
	for i, p := range api.posted[2:] {
		if p.Broadcast() != "" {
			t.Fatalf("posted[%d] に reply_broadcast が付いている", i+2)
		}
		if p.ThreadTS() != testMentionTS {
			t.Fatalf("posted[%d] がスレッド外に出ている: %q", i+2, p.ThreadTS())
		}
		if p.Text() == "" {
			t.Fatalf("posted[%d] の text フォールバックが空", i+2)
		}
	}
}

// #681 AC-5 / AC-6 / AC-7 / AC-8 / AC-22: 目次付きカード型（案 C）の中身。
// 固定 5 ブロック ＋ テーマごとに divider / section / rich_text / context。
func TestFocus_DigestBlocks_Cards(t *testing.T) {
	report := rankThemes(sampleDigest(t), false)
	blocks := digestBlocks(testJob(), nil, time.Now(), report)

	want := "header,context,rich_text," +
		strings.Repeat("divider,section,rich_text,context,", focusMaxThemes) +
		"divider,context"
	if got := strings.Join(blockTypes(blocks), ","); got != want {
		t.Fatalf("blocks = %q, want %q", got, want)
	}
	if n := len(blocks); n != focusDigestFixedBlocks+focusMaxThemes*focusBlocksPerTheme {
		t.Fatalf("blocks = %d, want %d", n, focusDigestFixedBlocks+focusMaxThemes*focusBlocksPerTheme)
	}

	raw := marshalBlocks(t, blocks)

	// AC-6: 目次は ordered list で、並び・件数が report.Focus と一致する。
	toc := richTextElements(t, raw[2])
	if got := strings.Join(richTextElementTypes(toc), ","); got != "rich_text_list" {
		t.Fatalf("目次ブロックの要素 = %q, want rich_text_list", got)
	}
	if toc[0]["style"] != "ordered" || toc[0]["indent"] != float64(0) {
		t.Fatalf("目次 = style %v / indent %v, want ordered / 0", toc[0]["style"], toc[0]["indent"])
	}
	wantTOC := []string{
		"タイミング\u3000QB, WR · 3 プレー",
		"縦の走り込み\u3000WR · 2 プレー",
		"セット前のコール\u30002 プレー", // AC-22: ポジションが空なら中黒ごと省く
	}
	if n := len(toc[0]["elements"].([]any)); n != len(wantTOC) {
		t.Fatalf("目次 = %d 件, want %d 件", n, len(wantTOC))
	}
	for i, want := range wantTOC {
		if got := richTextItemText(t, toc[0], i); got != want {
			t.Fatalf("目次[%d] = %q, want %q", i, got, want)
		}
		if !boldFirst(toc[0]["elements"].([]any)[i].(map[string]any)) {
			t.Fatalf("目次[%d] の label が太字になっていない", i)
		}
	}

	// カード 1 枚目（timing）。section は `N. title` ＋ 概要。
	if got := blockText(raw[4]); got != "*1. QB↔WR のタイミング*\nスナップ前に MOFO/MOFC を決めておらず、投げ先の判断がブレイク後になっている。" {
		t.Fatalf("カード見出し＋概要 = %q", got)
	}
	// AC-8: 末尾 context は ポジション · 件数（割合）· 引用。割合の分母は集計に
	// 残った全テーマの件数合計（3+2+2+1 = 8。3/8 = 38%）。
	if got := blockText(raw[6]); got != "QB, WR · 3 プレー（38%）\u3000·\u3000「Xがピタッと止まれてないのと、QBが待ちすぎ」" {
		t.Fatalf("カード末尾の context = %q", got)
	}

	// AC-7: 「やる」「やめる」は ✅ / 🚫 を先頭に置いた 1 つの bullet リスト。
	// 片方しか無いテーマでも空のリストを作らない。
	for _, c := range []struct {
		at    int // アクションの rich_text ブロックの位置
		items []string
	}{
		{5, []string{
			"✅ ブレイク 3 歩目でボールを離す",
			"✅ スナップ前に SF の目線で MOFO/MOFC を決める",
			"🚫 フラットを第一選択にして待つ",
			"🚫 投げ急いでリズムを崩す",
		}},
		{9, []string{"✅ 奥まで駆け抜けてから切る", "✅ 縦の 5 歩目まで減速しない"}},
		{13, []string{"🚫 黙ってセットする"}},
	} {
		list := richTextElements(t, raw[c.at])[0]
		if list["style"] != "bullet" || list["indent"] != float64(0) {
			t.Fatalf("blocks[%d] のリスト = style %v / indent %v, want bullet / 0",
				c.at, list["style"], list["indent"])
		}
		if n := len(list["elements"].([]any)); n != len(c.items) {
			t.Fatalf("blocks[%d] のアクション = %d 件, want %d 件", c.at, n, len(c.items))
		}
		for i, want := range c.items {
			if got := richTextItemText(t, list, i); got != want {
				t.Fatalf("blocks[%d] のアクション[%d] = %q, want %q", c.at, i, got, want)
			}
		}
	}
}

// #661 / #681 AC-9: 1 通目のブロック数は「固定 5 ＋ focus 件数 × 4」を超えない。
// focusDigestFixedBlocks / focusBlocksPerTheme の手勘定が digestBlocks の実装から
// ずれたら（context を 1 つ足した等）ここで落ちる。
func TestFocus_DigestBlocks_MaxThemes(t *testing.T) {
	report := focusReport{}
	for i := 0; i < focusMaxThemes; i++ {
		report.Focus = append(report.Focus, rankedTheme{
			focusTheme: focusTheme{Title: "テーマ", Label: "テーマ", Do: []string{"やる"}, Dont: []string{"やめる"}},
			Count:      1,
		})
	}
	blocks := digestBlocks(testJob(), nil, time.Now(), report)
	if got := len(blocks) - len(report.Focus)*focusBlocksPerTheme; got != focusDigestFixedBlocks {
		t.Fatalf("focus 以外のブロック = %d, want %d（focusDigestFixedBlocks とずれている）", got, focusDigestFixedBlocks)
	}
	if len(blocks) > focusMaxBlocksPerMessage {
		t.Fatalf("1 通目 = %d blocks, want <= %d", len(blocks), focusMaxBlocksPerMessage)
	}
}

// #681 AC-7: 「やる」「やめる」が両方 0 件のテーマは rich_text ごと省く
// （空の rich_text_list は Slack に invalid_blocks で弾かれる）。
func TestFocus_DigestBlocks_NoActions(t *testing.T) {
	report := focusReport{
		Focus: []rankedTheme{{
			focusTheme: focusTheme{Title: "タイトル", Label: "ラベル", Positions: []string{"QB"}},
			Count:      2,
		}},
		Stats: focusStats{Themes: []themeCount{{Key: "a", Count: 2}}},
	}
	blocks := digestBlocks(testJob(), nil, time.Now(), report)
	want := "header,context,rich_text,divider,section,context,divider,context"
	if got := strings.Join(blockTypes(blocks), ","); got != want {
		t.Fatalf("blocks = %q, want %q（アクションが無いテーマは rich_text を省く）", got, want)
	}
	if got := len(blocks); got != focusDigestFixedBlocks+focusBlocksPerTheme-1 {
		t.Fatalf("blocks = %d, want %d", got, focusDigestFixedBlocks+focusBlocksPerTheme-1)
	}
}

// #659 AC-6: 既定では summary の直後にチャート 1 通だけが続き、プレー別の一覧は出ない。
// `full` を付けたときだけチャートの後に一覧が続く。
func TestFocus_ChartAndDetails(t *testing.T) {
	run := func(t *testing.T, full bool) []sentMessage {
		t.Helper()
		api := focusFixture()
		bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{reply: digestJSON}}
		job := testJob()
		job.Full = full
		if err := bot.runFocus(t.Context(), job); err != nil {
			t.Fatalf("runFocus: %v", err)
		}
		return api.posted[1:] // posted[0] は受付メッセージ
	}

	t.Run("既定はチャートのみ", func(t *testing.T) {
		posted := run(t, false)
		if len(posted) != 2 {
			t.Fatalf("posted = %d, want 2（summary + チャート）", len(posted))
		}
		if got := strings.Join(posted[1].BlockTypes(), ","); got != "data_visualization,data_visualization" {
			t.Fatalf("チャート通の blocks = %q, want data_visualization × 2", got)
		}
		charts := posted[1].Blocks()
		if got := charts[0]["chart"].(map[string]any)["type"]; got != "pie" {
			t.Fatalf("1 個目の chart.type = %v, want pie", got)
		}
		if got := charts[1]["chart"].(map[string]any)["type"]; got != "bar" {
			t.Fatalf("2 個目の chart.type = %v, want bar", got)
		}
		if !strings.HasPrefix(posted[1].Text(), "課題の内訳: ") {
			t.Fatalf("チャート通の text = %q, want `課題の内訳: …`", posted[1].Text())
		}
	})

	t.Run("full ならチャートの後に一覧が続く", func(t *testing.T) {
		posted := run(t, true)
		if len(posted) != 3 {
			t.Fatalf("posted = %d, want 3（summary + チャート + 一覧）", len(posted))
		}
		// 一覧は見出しごとに section + rich_text。digestJSON は見出し 2 つぶん。
		if got := strings.Join(posted[2].BlockTypes(), ","); got != "section,rich_text,section,rich_text" {
			t.Fatalf("一覧の blocks = %q, want 見出し 2 つぶんの section,rich_text", got)
		}
	})
}

// AC-4: 詳細は見出し境界で分割され、1 通あたり 50 blocks 以下に収まる。
func TestDetailMessages_SplitAndLimits(t *testing.T) {
	t.Run("見出し境界で分割", func(t *testing.T) {
		report := focusReport{}
		for i := 0; i < 30; i++ { // 1 見出し = section + rich_text の 2 ブロック
			report.Plays = append(report.Plays, focusPlay{
				Headline: fmt.Sprintf("見出し%02d", i), Name: "プレー", Issue: "反省",
			})
		}
		msgs := detailMessages(report)
		if len(msgs) != 2 {
			t.Fatalf("msgs = %d, want 2（60 ブロックが 50 で割れる）", len(msgs))
		}
		for i, m := range msgs {
			if len(m.Blocks) > focusMaxBlocksPerMessage {
				t.Fatalf("msgs[%d] = %d blocks, want <= %d", i, len(m.Blocks), focusMaxBlocksPerMessage)
			}
			if _, ok := m.Blocks[0].(*slack.SectionBlock); !ok {
				t.Fatalf("msgs[%d] が見出し境界で始まっていない: %T", i, m.Blocks[0])
			}
		}
		if len(msgs[0].Blocks) != 50 || len(msgs[1].Blocks) != 10 {
			t.Fatalf("分割サイズ = %d / %d, want 50 / 10", len(msgs[0].Blocks), len(msgs[1].Blocks))
		}
	})

	t.Run("1 見出しが上限を超えるならプレー単位で割る", func(t *testing.T) {
		plays := make([]focusPlay, 0, focusMaxListItems*focusMaxBlocksPerMessage)
		for i := 0; i < focusMaxListItems*focusMaxBlocksPerMessage; i++ {
			plays = append(plays, focusPlay{Headline: "巨大な見出し", Name: fmt.Sprintf("プレー%04d", i)})
		}
		msgs := detailMessages(focusReport{Plays: plays})
		if len(msgs) < 2 {
			t.Fatalf("msgs = %d, want 2 以上（分割されていない）", len(msgs))
		}
		for i, m := range msgs {
			if len(m.Blocks) > focusMaxBlocksPerMessage {
				t.Fatalf("msgs[%d] = %d blocks, want <= %d", i, len(m.Blocks), focusMaxBlocksPerMessage)
			}
		}
		if !strings.Contains(msgs[1].Text, "（続き）") {
			t.Fatalf("分割後の見出しに（続き）が付いていない: %q", msgs[1].Text)
		}
	})

	t.Run("見出しは 3,000 文字・タイトルは 150 文字で切り詰める", func(t *testing.T) {
		long := strings.Repeat("あ", 4000)
		msgs := detailMessages(focusReport{Plays: []focusPlay{{Headline: long, Name: "プレー"}}})
		section := msgs[0].Blocks[0].(*slack.SectionBlock)
		if n := utf8.RuneCountInString(section.Text.Text); n > focusSectionRuneLimit {
			t.Fatalf("section text = %d 文字, want <= %d", n, focusSectionRuneLimit)
		}
		if got := truncateRunes(long, focusHeaderRuneLimit); utf8.RuneCountInString(got) != focusHeaderRuneLimit {
			t.Fatalf("truncateRunes = %d 文字, want %d", utf8.RuneCountInString(got), focusHeaderRuneLimit)
		}
	})

	// 見出しの空欄はチャートの x 軸と同じ「その他」に寄せる（寄せ先は focusHeadlineLabel が 1 箇所で決める）。
	t.Run("見出しが空でも 1 通にまとまる", func(t *testing.T) {
		msgs := detailMessages(focusReport{Plays: []focusPlay{{Name: "プレー", Issue: "反省"}}})
		if got := msgs[0].Text; got != focusUnknownHeadline {
			t.Fatalf("空の見出し = %q, want %q（チャートの x 軸と揃える）", got, focusUnknownHeadline)
		}
		if len(msgs) != 1 || len(msgs[0].Blocks) != 2 {
			t.Fatalf("msgs = %+v, want section + rich_text の 1 通", msgs)
		}
	})
}

// AC-4 補足: ThreadOnly では 1 通目にも broadcast を付けない。
func TestPostFocusMessages_ThreadOnly(t *testing.T) {
	api := newFakeSlackAPI()
	bot := Bot{SlackAPI: api}
	job := testJob()
	job.ThreadOnly = true

	msgs := focusMessages(job, nil, time.Now(), focusReport{
		Focus: []rankedTheme{{focusTheme: focusTheme{Title: "t"}}},
	})
	if err := bot.postFocusMessages(job, msgs); err != nil {
		t.Fatalf("postFocusMessages: %v", err)
	}
	for i, p := range api.posted {
		if p.Broadcast() != "" {
			t.Fatalf("posted[%d] に reply_broadcast が付いている（スレッド単体要約）", i)
		}
	}
	// ThreadOnly の header は「このスレッドの focus」（`8/26〜9/7 の focus` と違い
	// 助詞の前に空白を置かない）。focusRangeLabel の流用で崩さないための番人。
	if got := blockText(api.posted[0].Blocks()[0]); got != "このスレッドの focus" {
		t.Fatalf("header = %q, want このスレッドの focus", got)
	}
}

// AC-5: 要約入力に [見出し] / [プレー] は出さず、返信 0 の親は [投稿・返信なし] で渡す。
func TestRenderThreads_NoKindLabels(t *testing.T) {
	threads := []playThread{
		{Parent: parentMsg("100.000000", "@channel 明日の練習", 1),
			Replies: []slack.Message{replyMsg("101.000000", "U1", "了解")}},
		{Parent: parentMsg("200.000000", "17 JUICY p Na OZ WHAM", 0)},
	}
	got := renderThreads(threads, nil)

	for _, ng := range []string{"[見出し]", "[プレー]"} {
		if strings.Contains(got, ng) {
			t.Fatalf("要約入力に %s が残っている:\n%s", ng, got)
		}
	}
	if !strings.Contains(got, "[投稿] @channel 明日の練習") {
		t.Fatalf("返信のある投稿が [投稿] で渡っていない:\n%s", got)
	}
	if !strings.Contains(got, "[投稿・返信なし] 17 JUICY p Na OZ WHAM") {
		t.Fatalf("返信 0 の投稿が [投稿・返信なし] で渡っていない:\n%s", got)
	}
}

// #658 AC-4: プロンプトに禁止語・引用・対比・具体性の指示が入り、
// 集計を LLM に頼む文言（件数を数えろ・繰り返しを優先しろ）が残っていない。
func TestFocusSystemPrompt_Sharpness(t *testing.T) {
	prompt := focusSystemPrompt(false)
	for _, want := range []string{
		"意識する／徹底する／自信を持つ／コミュニケーション／連携／集中",
		"原文から 20〜40 文字をそのまま抜く",
		// #661: やる／やめる は概要の下に階層で並べるので、指示も 3 本に分かれる。
		"summary は何が起きていて何が原因かを 1〜2 文で書く",
		"do には次の練習でやることを動作で 1〜2 件",
		"dont にはやめることを 0〜2 件。無ければ空配列にする",
		"症状ではなく原因で切る",
		"issue はそのプレーで指摘された事実を 1 文で",
		// #681: 目次と pie 凡例に出す短い名前。title の指示は変えない。
		"label は目次と凡例に出す 14 文字以内の短い名詞句",
		"title は 1 行の見出し。誰が・どのプレーで・何が起きているかが分かる形にする",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("プロンプトに %q が無い:\n%s", want, prompt)
		}
	}
	// 順位と件数は Hub 側（rankThemes）の仕事。LLM に数えさせる指示を残さない。
	// stop / start は #661 で do / dont に置き換わったので、語ごと残さない。
	for _, ng := range []string{"count に根拠となったプレー数", "繰り返し出ている指摘を優先", "stop", "start", "detail"} {
		if strings.Contains(prompt, ng) {
			t.Fatalf("集計を LLM に頼む指示が残っている: %q", ng)
		}
	}
}

// AC-6: 対象が少ないときはテーマ数を絞る指示をプロンプトに入れる。
func TestFocusSystemPrompt_FewTargets(t *testing.T) {
	if !strings.Contains(focusSystemPrompt(true), "最大 3 個") {
		t.Fatal("few=true のプロンプトに「最大 3 個」が無い")
	}
	if !strings.Contains(focusSystemPrompt(false), "3〜7 個") {
		t.Fatal("few=false のプロンプトに「3〜7 個」が無い")
	}

	many := playThreads(focusFewTargetsThreshold + 1)
	few := many[:focusFewTargetsThreshold-1]

	threadOnly := testJob()
	threadOnly.ThreadOnly = true
	if !focusFewTargets(threadOnly, many) {
		t.Fatal("ThreadOnly が few 扱いになっていない")
	}
	if !focusFewTargets(testJob(), few) {
		t.Fatalf("プレー %d 件が few 扱いになっていない", len(few))
	}
	if focusFewTargets(testJob(), many) {
		t.Fatalf("プレー %d 件が few 扱いになっている", len(many))
	}

	// 実際に投げるプロンプトにも反映される。
	gpt := &fakeChatGPT{reply: digestJSON}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
	if _, err := bot.summarize(t.Context(), testJob(), few, nil); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if got := gpt.requests[0].System[0]; !strings.Contains(got, "最大 3 個") {
		t.Fatalf("対象が少ないのにプロンプトが 最大 3 個 になっていない:\n%s", got)
	}
}

// AC-4: focusReportSchema は Structured Outputs（strict）の制約を満たす。
// すべての object に additionalProperties:false があり、required が
// properties のキー集合と一致すること（strict では省略可能なフィールドを作れない）。
func TestFocusReportSchema_Strict(t *testing.T) {
	var walk func(path string, node map[string]any)
	walk = func(path string, node map[string]any) {
		switch node["type"] {
		case "object":
			props, ok := node["properties"].(map[string]any)
			if !ok {
				t.Fatalf("%s: object に properties が無い", path)
			}
			if node["additionalProperties"] != false {
				t.Fatalf("%s: additionalProperties:false が無い", path)
			}
			required, ok := node["required"].([]any)
			if !ok {
				t.Fatalf("%s: required が無い", path)
			}
			if len(required) != len(props) {
				t.Fatalf("%s: required %v が properties %d 件と一致しない", path, required, len(props))
			}
			for _, r := range required {
				name, _ := r.(string)
				if _, ok := props[name]; !ok {
					t.Fatalf("%s: required の %q が properties に無い", path, name)
				}
			}
			for name, child := range props {
				walk(path+"."+name, child.(map[string]any))
			}
		case "array":
			items, ok := node["items"].(map[string]any)
			if !ok {
				t.Fatalf("%s: array に items が無い", path)
			}
			walk(path+"[]", items)
		case "string", "integer", "number", "boolean":
		default:
			t.Fatalf("%s: 未知の type %v", path, node["type"])
		}
	}
	walk("focus_report", focusReportSchema)

	// focusDigest（Go 側の型）とキーが対応していること。
	props := focusReportSchema["properties"].(map[string]any)
	for _, key := range []string{"themes", "plays"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("schema に %q が無い", key)
		}
	}

	// #661 AC-1: themes は summary / do / dont を持ち、detail / stop / start は持たない。
	theme := props["themes"].(map[string]any)["items"].(map[string]any)
	themeProps := theme["properties"].(map[string]any)
	// #681 AC-2: label（目次・凡例に出す短い名前）を足した。
	for _, key := range []string{"key", "title", "label", "summary", "do", "dont", "positions", "quote"} {
		if _, ok := themeProps[key]; !ok {
			t.Fatalf("themes に %q が無い", key)
		}
	}
	if len(themeProps) != 8 {
		t.Fatalf("themes の properties = %v, want 8 件（detail / stop / start は消えている）", themeProps)
	}
	for _, key := range []string{"do", "dont"} {
		field := themeProps[key].(map[string]any)
		if field["type"] != "array" || field["items"].(map[string]any)["type"] != "string" {
			t.Fatalf("themes.%s = %+v, want array of string", key, field)
		}
	}
}

// AC-6: summarize は focus 用モデルと focus_digest schema で呼ぶ。
func TestSummarize_UsesStructuredOutputs(t *testing.T) {
	gpt := &fakeChatGPT{reply: digestJSON}
	bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}

	summary, err := bot.summarize(t.Context(), testJob(), playThreads(focusFewTargetsThreshold), nil)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.Report == nil || len(summary.Report.Focus) != focusMaxThemes {
		t.Fatalf("report が decode されていない: %+v", summary)
	}
	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	req := gpt.requests[0]
	if req.Model != chatModelFocus {
		t.Fatalf("model = %q, want %s", req.Model, chatModelFocus)
	}
	if req.Schema == nil {
		t.Fatal("Schema が nil（Structured Outputs になっていない）")
	}
	if req.Schema.Name != focusReportSchemaName {
		t.Fatalf("Schema.Name = %q, want %s", req.Schema.Name, focusReportSchemaName)
	}
	if !reflect.DeepEqual(req.Schema.Schema, focusReportSchema) {
		t.Fatal("Schema.Schema が focusReportSchema でない")
	}
	if len(req.System) != 1 || req.User == "" {
		t.Fatalf("System/User が期待どおりでない: %+v", req)
	}
}
