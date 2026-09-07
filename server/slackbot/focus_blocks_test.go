package slackbot

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/slack-go/slack"
)

// digestJSON は LLM が返す構造化出力（focus 3 件・sections 1 件）。
const digestJSON = `{
  "focus": [
    {"title":"QB↔WR のタイミング","detail":"WR はブレイクを明確に、QB は早めのリリース","positions":["QB","WR"],"count":12,"plays":["プレーA","プレーB"]},
    {"title":"縦の走り込み","detail":"足を止めず奥まで駆け抜ける","positions":["WR"],"count":8,"plays":["プレーA"]},
    {"title":"コールの徹底","detail":"セット前に声を出して確認する","positions":[],"count":3,"plays":["プレーB"]}
  ],
  "sections": [
    {"headline":"GL Drive1","plays":[
      {"name":"プレーA","points":["奥まで駆け抜ける","キャッチミスの原因を特定"]},
      {"name":"プレーB","points":[]}
    ]}
  ]
}`

func sampleDigest(t *testing.T) focusDigest {
	t.Helper()
	api := focusFixture()
	gpt := &fakeChatGPT{reply: digestJSON}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}
	summary, err := bot.summarize(t.Context(), testJob(), []playThread{{Parent: parentMsg("1", "x", 1)}}, nil)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.Digest == nil {
		t.Fatal("digest が decode されていない")
	}
	return *summary.Digest
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

// AC-1: JSON を focusDigest に decode し、focus の順序・count・plays を保つ。
func TestSummarize_DecodesDigest(t *testing.T) {
	digest := sampleDigest(t)

	if len(digest.Focus) != 3 {
		t.Fatalf("focus = %d 件, want 3", len(digest.Focus))
	}
	wantTitles := []string{"QB↔WR のタイミング", "縦の走り込み", "コールの徹底"}
	for i, want := range wantTitles {
		if got := digest.Focus[i].Title; got != want {
			t.Fatalf("focus[%d].Title = %q, want %q（順序が保たれていない）", i, got, want)
		}
	}
	if got := digest.Focus[0].Count; got != 12 {
		t.Fatalf("focus[0].Count = %d, want 12", got)
	}
	if got := strings.Join(digest.Focus[0].Plays, ","); got != "プレーA,プレーB" {
		t.Fatalf("focus[0].Plays = %q", got)
	}
	if got := strings.Join(digest.Focus[0].Positions, ","); got != "QB,WR" {
		t.Fatalf("focus[0].Positions = %q", got)
	}
	if len(digest.Sections) != 1 || digest.Sections[0].Headline != "GL Drive1" {
		t.Fatalf("sections = %+v, want GL Drive1 の 1 件", digest.Sections)
	}
	if len(digest.Sections[0].Plays) != 2 || digest.Sections[0].Plays[0].Name != "プレーA" {
		t.Fatalf("sections[0].Plays = %+v", digest.Sections[0].Plays)
	}
}

// AC-2: コードフェンス付きでも decode でき、壊れた JSON は
// エラーにせず平文フォールバックへ倒れ、その旨が log に出る。
func TestSummarize_CodeFenceAndBrokenJSON(t *testing.T) {
	threads := []playThread{{Parent: parentMsg("100.000000", "プレーA", 1),
		Replies: []slack.Message{replyMsg("101.000000", "U1", "反省1")}}}

	t.Run("コードフェンス付き", func(t *testing.T) {
		gpt := &fakeChatGPT{reply: "```json\n" + digestJSON + "\n```"}
		bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
		summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
		if err != nil {
			t.Fatalf("summarize: %v", err)
		}
		if summary.Digest == nil || len(summary.Digest.Focus) != 3 {
			t.Fatalf("フェンス付き JSON が decode されていない: %+v", summary)
		}
	})

	t.Run("改行の無いコードフェンス", func(t *testing.T) {
		gpt := &fakeChatGPT{reply: "```json " + strings.Join(strings.Fields(digestJSON), " ") + " ```"}
		bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
		summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
		if err != nil {
			t.Fatalf("summarize: %v", err)
		}
		if summary.Digest == nil || len(summary.Digest.Focus) != 3 {
			t.Fatalf("1 行のフェンス付き JSON が decode されていない: %+v", summary)
		}
	})

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
		if summary.Digest != nil {
			t.Fatalf("壊れた JSON なのに digest が返っている: %+v", summary.Digest)
		}
		if summary.Text != "*プレーA* 縦の走り込みを揃える。" {
			t.Fatalf("平文フォールバックの本文が失われている: %q", summary.Text)
		}
		if !strings.Contains(buf.String(), "digest parse failed") {
			t.Fatalf("フォールバックが log に残っていない: %q", buf.String())
		}
	})

	t.Run("focus 0 件も平文フォールバック", func(t *testing.T) {
		gpt := &fakeChatGPT{reply: `{"focus":[],"sections":[]}`}
		bot := Bot{SlackAPI: newFakeSlackAPI(), ChatGPT: gpt}
		summary, err := bot.summarize(t.Context(), testJob(), threads, nil)
		if err != nil {
			t.Fatalf("summarize: %v", err)
		}
		if summary.Digest != nil {
			t.Fatal("focus 0 件で digest を返している（空の rich_text_list は invalid_blocks になる）")
		}
	})
}

// AC-3: 1 通目は header → context → rich_text(ordered) → divider → context の
// 5 ブロックで、チャンネルにも出す（reply_broadcast=true）。text は空にしない。
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
	if got := strings.Join(digest.BlockTypes(), ","); got != "header,context,rich_text,divider,context" {
		t.Fatalf("1 通目の blocks = %q, want header,context,rich_text,divider,context", got)
	}
	if got := digest.Broadcast(); got != "true" {
		t.Fatalf("1 通目の reply_broadcast = %q, want true", got)
	}
	if digest.Text() == "" {
		t.Fatal("通知用の text フォールバックが空（モバイル通知に何も出ない）")
	}
	if !strings.Contains(digest.Text(), "1. QB↔WR のタイミング") {
		t.Fatalf("text フォールバックに focus 1 点目が入っていない: %q", digest.Text())
	}

	blocks := digest.Blocks()
	if n := utf8.RuneCountInString(blockText(blocks[0])); n > focusHeaderRuneLimit {
		t.Fatalf("header が %d 文字（上限 %d）", n, focusHeaderRuneLimit)
	}
	if got := blockText(blocks[1]); !strings.Contains(got, "2 プレー / 4 件の反省から") {
		t.Fatalf("context の件数メタが期待どおりでない: %q", got)
	}

	list := blocks[2]["elements"].([]any)[0].(map[string]any)
	if list["type"] != "rich_text_list" || list["style"] != "ordered" {
		t.Fatalf("focus が番号付きリストになっていない: %+v", list)
	}
	if n := len(list["elements"].([]any)); n != 3 {
		t.Fatalf("リスト項目 = %d, want 3（focus 件数）", n)
	}

	// 詳細は同じスレッドに broadcast 無しで続く。
	for i, p := range api.posted[2:] {
		if p.Broadcast() != "" {
			t.Fatalf("詳細 posted[%d] に reply_broadcast が付いている", i+2)
		}
		if p.ThreadTS() != testMentionTS {
			t.Fatalf("詳細 posted[%d] がスレッド外に出ている: %q", i+2, p.ThreadTS())
		}
		if p.Text() == "" {
			t.Fatalf("詳細 posted[%d] の text フォールバックが空", i+2)
		}
	}
	if got := strings.Join(api.posted[2].BlockTypes(), ","); got != "section,rich_text" {
		t.Fatalf("詳細の blocks = %q, want section,rich_text", got)
	}
}

// AC-4: 詳細は見出し境界で分割され、1 通あたり 50 blocks 以下に収まる。
func TestDetailMessages_SplitAndLimits(t *testing.T) {
	t.Run("見出し境界で分割", func(t *testing.T) {
		digest := focusDigest{}
		for i := 0; i < 30; i++ { // 1 見出し = section + rich_text の 2 ブロック
			digest.Sections = append(digest.Sections, focusSection{
				Headline: fmt.Sprintf("見出し%02d", i),
				Plays:    []focusPlay{{Name: "プレー", Points: []string{"反省"}}},
			})
		}
		msgs := detailMessages(digest)
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
			plays = append(plays, focusPlay{Name: fmt.Sprintf("プレー%04d", i)})
		}
		msgs := detailMessages(focusDigest{Sections: []focusSection{{Headline: "巨大な見出し", Plays: plays}}})
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
		msgs := detailMessages(focusDigest{Sections: []focusSection{{
			Headline: long, Plays: []focusPlay{{Name: "プレー"}},
		}}})
		section := msgs[0].Blocks[0].(*slack.SectionBlock)
		if n := utf8.RuneCountInString(section.Text.Text); n > focusSectionRuneLimit {
			t.Fatalf("section text = %d 文字, want <= %d", n, focusSectionRuneLimit)
		}
		if got := truncateRunes(long, focusHeaderRuneLimit); utf8.RuneCountInString(got) != focusHeaderRuneLimit {
			t.Fatalf("truncateRunes = %d 文字, want %d", utf8.RuneCountInString(got), focusHeaderRuneLimit)
		}
	})

	t.Run("見出しが空でも 1 通にまとまる", func(t *testing.T) {
		msgs := detailMessages(focusDigest{Sections: []focusSection{{
			Plays: []focusPlay{{Name: "プレー", Points: []string{"反省"}}},
		}}})
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

	msgs := focusMessages(job, nil, time.Now(), focusDigest{Focus: []focusItem{{Title: "t"}}})
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

// AC-6: 対象が少ないときは focus を 1〜3 点に絞る指示をプロンプトに入れる。
func TestFocusSystemPrompt_FewTargets(t *testing.T) {
	if !strings.Contains(focusSystemPrompt(true), "1〜3 点") {
		t.Fatal("few=true のプロンプトに「1〜3 点」が無い")
	}
	if !strings.Contains(focusSystemPrompt(false), "3〜5 点") {
		t.Fatal("few=false のプロンプトに「3〜5 点」が無い")
	}

	play := func(ts string) playThread {
		return playThread{Parent: parentMsg(ts, "プレー", 1),
			Replies: []slack.Message{replyMsg(ts+"1", "U1", "反省")}}
	}
	many := []playThread{}
	for i := 0; i < focusFewTargetsThreshold+1; i++ {
		many = append(many, play(fmt.Sprintf("%03d.000000", 100+i)))
	}
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
	if got := gpt.requests[0].Messages[0].Content; !strings.Contains(got, "1〜3 点") {
		t.Fatalf("対象が少ないのにプロンプトが 1〜3 点になっていない:\n%s", got)
	}
}
