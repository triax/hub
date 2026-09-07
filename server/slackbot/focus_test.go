package slackbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
)

const testMentionTS = "350.000000"

// focusDonePattern は完了メッセージ末尾の所要時間（`（42 秒）` / `（1 分 42 秒）`）。
var focusDonePattern = regexp.MustCompile(`（\d+ 秒）$|（\d+ 分( \d+ 秒)?）$`)

// focusFixture は「history 2 ページ / replies 2 ページ」のチャンネルを組み立てる。
//
//	100 プレーA（返信 3 件 = 2 ページに分かれる）
//	200 見出し （返信 0 件）
//	300 プレーB（返信 1 件）
//	250 bot の投稿（除外対象）
//	350 要約を起動したメンション自身（除外対象）
func focusFixture() *fakeSlackAPI {
	api := newFakeSlackAPI()

	mention := parentMsg(testMentionTS, "<@BOT> focus 8/8", 1)
	api.historyPages = []*slack.GetConversationHistoryResponse{
		// history は新しい順に返る
		historyPage("h2",
			mention,
			parentMsg("300.000000", "プレーB", 1),
			botMsg("250.000000", "bot のお知らせ"),
			parentMsg("200.000000", "GL Drive1", 0),
		),
		historyPage("", parentMsg("100.000000", "プレーA", 3)),
	}

	api.repliesPages["100.000000"] = [][]slack.Message{
		{parentMsg("100.000000", "プレーA", 3), replyMsg("101.000000", "U1", "反省1"), replyMsg("102.000000", "U2", "反省2")},
		{replyMsg("103.000000", "U1", "反省3"), botMsg("104.000000", "bot の返信")},
	}
	api.repliesPages["300.000000"] = [][]slack.Message{
		{
			parentMsg("300.000000", "プレーB", 1),
			replyMsg("301.000000", "U3", "反省4"),
			// メンション自身がスレッド返信として現れるケース
			replyMsg(testMentionTS, "U9", "<@BOT> focus 8/8"),
		},
	}
	return api
}

func testJob() focusJob {
	return focusJob{Channel: "C1", MentionTS: testMentionTS, Oldest: 0}
}

// AC-1: history 2 ページ・replies 2 ページを全件取得し、古い順に並び、
// 親自身・bot・起動メンションを含まない。
func TestCollectThreads_Paging(t *testing.T) {
	api := focusFixture()
	bot := Bot{SlackAPI: api}

	threads, err := bot.collectChannelThreads(testJob())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(threads) != 3 {
		t.Fatalf("threads = %d, want 3 (%v)", len(threads), threads)
	}
	if api.historyCalls != 2 {
		t.Fatalf("history calls = %d, want 2 (ページングされていない)", api.historyCalls)
	}

	wantOrder := []string{"100.000000", "200.000000", "300.000000"}
	for i, want := range wantOrder {
		if got := threads[i].Parent.Timestamp; got != want {
			t.Fatalf("threads[%d].Parent.Timestamp = %s, want %s（古い順に並んでいない）", i, got, want)
		}
	}
	for _, th := range threads {
		if th.Parent.Timestamp == testMentionTS || th.Parent.BotID != "" {
			t.Fatalf("除外されるべき親が含まれている: %+v", th.Parent)
		}
	}

	if got := joined(texts(threads[0].Replies)); got != "反省1,反省2,反省3" {
		t.Fatalf("threads[0].Replies = %q, want 反省1,反省2,反省3（親自身・bot・メンションを除いた全返信）", got)
	}
	if got := joined(texts(threads[2].Replies)); got != "反省4" {
		t.Fatalf("threads[2].Replies = %q, want 反省4", got)
	}
	if len(threads[1].Replies) != 0 {
		t.Fatalf("返信 0 件の親に返信が付いている: %v", threads[1].Replies)
	}
}

// AC-2: 返信 0 件の親は見出し、1 件以上はプレー。出力では見出しが元の順で区切りとして現れる。
func TestCollectThreads_HeadlineClassification(t *testing.T) {
	api := focusFixture()
	bot := Bot{SlackAPI: api}

	threads, err := bot.collectChannelThreads(testJob())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if threads[0].IsHeadline() {
		t.Fatal("返信のある親が見出しと判定されている")
	}
	if !threads[1].IsHeadline() {
		t.Fatal("返信 0 件の親が見出しと判定されていない")
	}

	// 要約入力ではプレー／見出しを断定せず「返信なし」という事実だけを渡す（#653 AC-5）。
	rendered := renderThreads(threads, nil)
	wantSequence := []string{"[投稿] プレーA", "[投稿・返信なし] GL Drive1", "[投稿] プレーB"}
	pos := -1
	for _, want := range wantSequence {
		at := strings.Index(rendered, want)
		if at < 0 {
			t.Fatalf("要約入力に %q が現れない:\n%s", want, rendered)
		}
		if at <= pos {
			t.Fatalf("要約入力の並び順が崩れている（%q）:\n%s", want, rendered)
		}
		pos = at
	}
	if !strings.Contains(rendered, "- U1: 反省1") {
		t.Fatalf("返信本文が要約入力に含まれない:\n%s", rendered)
	}
}

// AC-2 補足: <@Uxxxx> は表示名に置換される（解決できなければ原文のまま）。
func TestRenderThreads_ResolvesMentions(t *testing.T) {
	threads := []playThread{{
		Parent:  parentMsg("100.000000", "プレー <@U1> 参照", 1),
		Replies: []slack.Message{replyMsg("101.000000", "U2", "<@U1> と <@UNKNOWN> へ")},
	}}
	resolve := func(id string) string {
		if id == "U1" {
			return "丸岡壮輝"
		}
		return id
	}
	got := renderThreads(threads, resolve)
	if !strings.Contains(got, "プレー @丸岡壮輝 参照") {
		t.Fatalf("親のメンションが置換されていない:\n%s", got)
	}
	if !strings.Contains(got, "@丸岡壮輝 と <@UNKNOWN> へ") {
		t.Fatalf("返信のメンション置換が期待どおりでない:\n%s", got)
	}
}

// AC-3: 期間指定の解釈。
func TestParseFocusSince(t *testing.T) {
	now := time.Date(2026, 9, 7, 13, 45, 0, 0, server.ServiceLocation)

	cases := []struct {
		name string
		args []string
		want time.Time
	}{
		{"M/D は今年", []string{"8/8"}, time.Date(2026, 8, 8, 0, 0, 0, 0, server.ServiceLocation)},
		{"未来日の M/D は前年", []string{"12/25"}, time.Date(2025, 12, 25, 0, 0, 0, 0, server.ServiceLocation)},
		{"YYYY-MM-DD", []string{"2026-08-08"}, time.Date(2026, 8, 8, 0, 0, 0, 0, server.ServiceLocation)},
		{"Nd", []string{"3d"}, time.Date(2026, 9, 4, 0, 0, 0, 0, server.ServiceLocation)},
		{"未指定は 12 日", nil, time.Date(2026, 8, 26, 0, 0, 0, 0, server.ServiceLocation)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseFocusSince(c.args, now)
			if err != nil {
				t.Fatalf("parseFocusSince(%v): %v", c.args, err)
			}
			if !got.Equal(c.want) {
				t.Fatalf("parseFocusSince(%v) = %s, want %s", c.args, got, c.want)
			}
			if h, m, s := got.Clock(); h+m+s != 0 {
				t.Fatalf("境界が 00:00 になっていない: %s", got)
			}
		})
	}

	if _, err := parseFocusSince([]string{"にゃーん"}, now); err == nil {
		t.Fatal("読めない引数がエラーにならない")
	}
}

// AC-3: 引数は Oldest（unix 秒）として history に渡る。
func TestFetchParents_PassesOldest(t *testing.T) {
	api := focusFixture()
	bot := Bot{SlackAPI: api}
	now := time.Date(2026, 9, 7, 13, 45, 0, 0, server.ServiceLocation)

	job, err := newFocusJob([]string{"8/8"}, now, "C1", testMentionTS, "")
	if err != nil {
		t.Fatalf("newFocusJob: %v", err)
	}
	want := time.Date(2026, 8, 8, 0, 0, 0, 0, server.ServiceLocation).Unix()
	if job.Oldest != want {
		t.Fatalf("job.Oldest = %d, want %d", job.Oldest, want)
	}
	if _, err := bot.fetchParents(job); err != nil {
		t.Fatalf("fetchParents: %v", err)
	}
	if got := api.historyParams[0].Oldest; got != fmt.Sprint(want) {
		t.Fatalf("history.Oldest = %q, want %q", got, fmt.Sprint(want))
	}
}

// AC-3: スレッド内メンション + 引数なしは history を呼ばず replies だけを見る。
func TestCollectThreads_ThreadOnly(t *testing.T) {
	api := newFakeSlackAPI()
	api.repliesPages["50.000000"] = [][]slack.Message{{
		parentMsg("50.000000", "プレーX", 2),
		replyMsg("51.000000", "U1", "反省X"),
		replyMsg(testMentionTS, "U9", "<@BOT> focus"),
	}}
	bot := Bot{SlackAPI: api}

	job, err := newFocusJob(nil, time.Now(), "C1", testMentionTS, "50.000000")
	if err != nil {
		t.Fatalf("newFocusJob: %v", err)
	}
	if !job.ThreadOnly {
		t.Fatal("スレッド内メンション + 引数なしが ThreadOnly にならない")
	}

	threads, err := bot.collectSingleThread(job)
	if err != nil {
		t.Fatalf("collectSingleThread: %v", err)
	}
	if api.historyCalls != 0 {
		t.Fatalf("history calls = %d, want 0（スレッド単体要約で history を呼んでいる）", api.historyCalls)
	}
	if len(threads) != 1 || threads[0].Parent.Text != "プレーX" {
		t.Fatalf("threads = %+v, want 対象スレッド 1 本", threads)
	}
	if got := joined(texts(threads[0].Replies)); got != "反省X" {
		t.Fatalf("Replies = %q, want 反省X（親とメンションを除く）", got)
	}
}

// #659 AC-6: `full` は期間指定と独立に解釈する。
func TestNewFocusJob_Full(t *testing.T) {
	now := time.Date(2026, 9, 7, 13, 45, 0, 0, server.ServiceLocation)
	want12d := time.Date(2026, 8, 26, 0, 0, 0, 0, server.ServiceLocation).Unix()

	cases := []struct {
		name       string
		args       []string
		threadTS   string
		wantFull   bool
		wantThread bool
		wantOldest int64
	}{
		{"full なし", []string{"12d"}, "", false, false, want12d},
		{"期間 + full", []string{"12d", "full"}, "", true, false, want12d},
		{"full + 期間（順不同）", []string{"full", "12d"}, "", true, false, want12d},
		{"大文字も拾う", []string{"12d", "FULL"}, "", true, false, want12d},
		{"引数が full だけなら期間は既定", []string{"full"}, "", true, false, want12d},
		{"スレッド内の full は ThreadOnly のまま", []string{"full"}, "50.000000", true, true, 0},
		{"スレッド内で引数なし", nil, "50.000000", false, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job, err := newFocusJob(c.args, now, "C1", testMentionTS, c.threadTS)
			if err != nil {
				t.Fatalf("newFocusJob(%v): %v", c.args, err)
			}
			if job.Full != c.wantFull {
				t.Fatalf("Full = %v, want %v", job.Full, c.wantFull)
			}
			if job.ThreadOnly != c.wantThread {
				t.Fatalf("ThreadOnly = %v, want %v", job.ThreadOnly, c.wantThread)
			}
			if job.Oldest != c.wantOldest {
				t.Fatalf("Oldest = %d, want %d", job.Oldest, c.wantOldest)
			}
		})
	}

	// Cloud Tasks の payload を跨いでも落ちない（Webhook → ワーカーは JSON 越し）。
	if !strings.Contains(mustMarshal(t, focusJob{Full: true}), `"full":true`) {
		t.Fatal("focusJob の JSON に full が乗っていない（ワーカーに伝わらない）")
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(buf)
}

// AC-4: 12,000 文字の要約が 3,500 文字以下のチャンクに、行を割らずに分割される。
func TestChunkLines(t *testing.T) {
	lines := []string{}
	for i := 0; len(strings.Join(lines, "\n")) < 12000; i++ {
		lines = append(lines, fmt.Sprintf("%03d 行目の要約テキストです。次の練習で意識することを書きます。", i))
	}
	input := strings.Join(lines, "\n")

	chunks := chunkLines(input, focusChunkSize)
	if len(chunks) < 2 {
		t.Fatalf("chunks = %d, want 2 以上（分割されていない）", len(chunks))
	}
	for i, c := range chunks {
		if n := utf8.RuneCountInString(c); n > focusChunkSize {
			t.Fatalf("chunks[%d] = %d 文字, want <= %d", i, n, focusChunkSize)
		}
		// 行頭で始まる = 元のいずれかの行と一致する先頭行を持つ
		head := strings.SplitN(c, "\n", 2)[0]
		found := false
		for _, l := range lines {
			if l == head {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("chunks[%d] が行途中から始まっている: %q", i, head)
		}
	}
	// 分割で本文が欠けたり増えたりしない
	if got := strings.Join(chunks, "\n"); got != input {
		t.Fatal("チャンクを連結しても元のテキストに戻らない")
	}
}

// AC-4: reply_broadcast は 1 チャンク目だけ。スレッド単体要約では付かない。
func TestPostSummary_Broadcast(t *testing.T) {
	chunks := []string{"1 つ目", "2 つ目", "3 つ目"}

	t.Run("チャンネル要約", func(t *testing.T) {
		api := newFakeSlackAPI()
		bot := Bot{SlackAPI: api}
		if err := bot.postSummary(testJob(), chunks); err != nil {
			t.Fatalf("postSummary: %v", err)
		}
		if len(api.posted) != 3 {
			t.Fatalf("posted = %d, want 3", len(api.posted))
		}
		if got := api.posted[0].Broadcast(); got != "true" {
			t.Fatalf("1 チャンク目の reply_broadcast = %q, want true", got)
		}
		for i, p := range api.posted[1:] {
			if p.Broadcast() != "" {
				t.Fatalf("posted[%d] に reply_broadcast が付いている", i+1)
			}
		}
		for i, p := range api.posted {
			if p.ThreadTS() != testMentionTS {
				t.Fatalf("posted[%d].thread_ts = %q, want %q", i, p.ThreadTS(), testMentionTS)
			}
		}
	})

	t.Run("スレッド単体要約", func(t *testing.T) {
		api := newFakeSlackAPI()
		bot := Bot{SlackAPI: api}
		job := testJob()
		job.ThreadOnly = true
		if err := bot.postSummary(job, chunks); err != nil {
			t.Fatalf("postSummary: %v", err)
		}
		for i, p := range api.posted {
			if p.Broadcast() != "" {
				t.Fatalf("posted[%d] に reply_broadcast が付いている（スレッド単体要約）", i)
			}
		}
	})
}

// 要約は既定で 1 プロンプト 1 回。UX（👀 / 受付 / 進捗 / 完了 ✅）も併せて確認する。
func TestFocus_EndToEnd(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: "*プレーA* 縦の走り込みを揃える。\n[見出し] GL Drive1\n*プレーB* 声を出す。"}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runFocus(t.Context(), testJob()); err != nil {
		t.Fatalf("runFocus: %v", err)
	}

	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	if gpt.requests[0].Model != chatModelFocus {
		t.Fatalf("model = %q, want %s", gpt.requests[0].Model, chatModelFocus)
	}
	if len(api.posted) < 2 {
		t.Fatalf("posted = %d, want 受付メッセージ + 要約", len(api.posted))
	}
	if !strings.Contains(api.posted[0].Text(), "2 プレー（見出し 1 件）") {
		t.Fatalf("受付メッセージが期待どおりでない: %q", api.posted[0].Text())
	}
	if !strings.Contains(api.posted[1].Text(), "2 プレー / 4 件の返信を要約") {
		t.Fatalf("要約のメタ行が期待どおりでない: %q", api.posted[1].Text())
	}
	// #657: 完了時も期間・件数・所要時間を残す（「✅ 完了」で上書きしない）。
	if len(api.updated) == 0 {
		t.Fatalf("完了時に受付メッセージが更新されていない: %+v", api.updated)
	}
	done := api.updated[len(api.updated)-1].Text()
	if !strings.HasPrefix(done, "✅ ") || !strings.Contains(done, "2 プレー / 4 件の反省を読みました") {
		t.Fatalf("完了メッセージに期間・件数が残っていない: %q", done)
	}
	if !focusDonePattern.MatchString(done) {
		t.Fatalf("完了メッセージに所要時間が残っていない: %q", done)
	}
	// 👀 は onMentionFocus（受付側）で付くので、ワーカーは外して ✅ を付けるだけ。
	if joined(api.added) != "white_check_mark" || joined(api.removed) != "eyes" {
		t.Fatalf("リアクションの遷移が期待どおりでない: added=%v removed=%v", api.added, api.removed)
	}
}

// #657 AC-1 / AC-2: 完了メッセージ（meta reply の最終形）に期間・件数・所要時間が残る。
func TestFocusDoneText(t *testing.T) {
	now := time.Date(2026, 9, 7, 13, 45, 0, 0, server.ServiceLocation)
	since := time.Date(2026, 8, 26, 0, 0, 0, 0, server.ServiceLocation)

	// プレー 2 本（返信 3 + 1）と見出し 1 本。countThreadKinds の実数が入る。
	threads := []playThread{
		{Parent: parentMsg("100.000000", "プレーA", 3), Replies: []slack.Message{
			replyMsg("101.000000", "U1", "反省1"),
			replyMsg("102.000000", "U2", "反省2"),
			replyMsg("103.000000", "U1", "反省3"),
		}},
		{Parent: parentMsg("200.000000", "GL Drive1", 0)},
		{Parent: parentMsg("300.000000", "プレーB", 1), Replies: []slack.Message{
			replyMsg("301.000000", "U3", "反省4"),
		}},
	}

	channel := focusJob{Channel: "C1", MentionTS: testMentionTS, Oldest: since.Unix()}
	threadOnly := focusJob{Channel: "C1", MentionTS: testMentionTS, ThreadOnly: true}

	cases := []struct {
		name    string
		job     focusJob
		elapsed time.Duration
		want    string
	}{
		{"チャンネル要約・秒", channel, 42 * time.Second, "✅ 8/26〜9/7 の 2 プレー / 4 件の反省を読みました（42 秒）"},
		{"チャンネル要約・分秒", channel, 102 * time.Second, "✅ 8/26〜9/7 の 2 プレー / 4 件の反省を読みました（1 分 42 秒）"},
		{"チャンネル要約・丁度 2 分", channel, 120 * time.Second, "✅ 8/26〜9/7 の 2 プレー / 4 件の反省を読みました（2 分）"},
		{"スレッド単体要約", threadOnly, 18 * time.Second, "✅ このスレッドの 4 件の返信を読みました（18 秒）"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := focusDoneText(c.job, threads, now, c.elapsed); got != c.want {
				t.Fatalf("focusDoneText = %q, want %q", got, c.want)
			}
		})
	}

	// ミリ秒は秒に丸める（`0.4 秒` のような表示にしない）。
	if got := focusElapsedLabel(1500 * time.Millisecond); got != "2 秒" {
		t.Fatalf("focusElapsedLabel(1.5s) = %q, want 2 秒", got)
	}
}

// 対象 0 件のときは要約を試みず、その旨だけを返す。
func TestFocus_NoTargets(t *testing.T) {
	api := newFakeSlackAPI()
	gpt := &fakeChatGPT{}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runFocus(t.Context(), testJob()); err != nil {
		t.Fatalf("runFocus: %v", err)
	}
	if len(gpt.requests) != 0 {
		t.Fatal("対象 0 件なのに ChatGPT を呼んでいる")
	}
	if len(api.posted) != 1 || api.posted[0].Text() != focusEmptyMessage {
		t.Fatalf("posted = %+v, want %q のみ", api.posted, focusEmptyMessage)
	}
}

// 失敗はスレッドにそのまま返し、👀 を外す（黙って失敗しない）。
// ChatGPT が未設定の環境でも panic せず、スレッドにエラーを返す。
// （NewOpenAIChat はキーが無いと nil を返すので、要約もガードを通す必要がある）
func TestFocus_WithoutChatGPT(t *testing.T) {
	api := focusFixture()
	bot := Bot{SlackAPI: api}

	err := bot.runFocus(t.Context(), testJob())
	if !errors.Is(err, ErrNoChatGPT) {
		t.Fatalf("err = %v, want ErrNoChatGPT", err)
	}
	last := api.posted[len(api.posted)-1]
	if !strings.Contains(last.Text(), ErrNoChatGPT.Error()) {
		t.Fatalf("エラーがスレッドに返っていない: %q", last.Text())
	}
	if joined(api.removed) != "eyes" {
		t.Fatalf("失敗時に 👀 が外れていない: %v", api.removed)
	}
}

func TestFocus_ReportsError(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{err: fmt.Errorf("missing_scope")}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runFocus(t.Context(), testJob()); err == nil {
		t.Fatal("要約失敗が error として返らない")
	}
	last := api.posted[len(api.posted)-1]
	if !strings.Contains(last.Text(), "missing_scope") {
		t.Fatalf("エラーがスレッドに返っていない: %q", last.Text())
	}
	if last.ThreadTS() != testMentionTS {
		t.Fatalf("エラーがメンションのスレッド外に投稿されている: %q", last.ThreadTS())
	}
	if joined(api.removed) != "eyes" {
		t.Fatalf("失敗時に 👀 が外れていない: %v", api.removed)
	}
	if len(api.added) != 0 {
		t.Fatalf("失敗したのに ✅ が付いている: %v", api.added)
	}
}

// 入力が文脈長に収まる限り分割しない。超えたときだけ塊に分ける。
func TestSplitThreadsForPrompt(t *testing.T) {
	threads := []playThread{
		{Parent: parentMsg("1", strings.Repeat("あ", 100), 1)},
		{Parent: parentMsg("2", strings.Repeat("い", 100), 1)},
		{Parent: parentMsg("3", strings.Repeat("う", 100), 1)},
	}
	if got := splitThreadsForPrompt(threads, focusPromptRuneBudget); len(got) != 1 {
		t.Fatalf("groups = %d, want 1（閾値以下なので分割しない）", len(got))
	}
	groups := splitThreadsForPrompt(threads, 150)
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(groups))
	}
}

// Cloud Tasks の task ID には `.` を使えないので置換する。
func TestFocusTaskName(t *testing.T) {
	got := focusTaskName(focusJob{Channel: "C123", MentionTS: "1757000000.123456"})
	if got != "focus-C123-1757000000-123456" {
		t.Fatalf("focusTaskName = %q", got)
	}
	if strings.Contains(got, ".") {
		t.Fatalf("task ID に `.` が残っている: %q", got)
	}
}

// collectChannelThreads は focus() が非スレッド時に踏む経路（fetchParents → expandThreads）を
// テストからまとめて呼ぶためのヘルパ。本番の focus() は受付メッセージを挟むため
// この 2 つを直接呼んでいる。
func (bot Bot) collectChannelThreads(job focusJob) ([]playThread, error) {
	parents, err := bot.fetchParents(job)
	if err != nil {
		return nil, err
	}
	return bot.expandThreads(job, parents, nil)
}

// 進捗の分子は「返信を取りに行ったプレー」だけで数える。見出しを混ぜると
// 「42 プレー中 45 件…」のように分子が分母を超えてしまう（AC-10 の UX に直結）。
func TestExpandThreads_ProgressSkipsHeadlines(t *testing.T) {
	api := newFakeSlackAPI()
	parents := []slack.Message{}
	for i := 0; i < 12; i++ {
		ts := fmt.Sprintf("%03d.000000", 100+i)
		if i%2 == 0 { // 偶数番は見出し（返信 0 件）
			parents = append(parents, parentMsg(ts, "見出し", 0))
			continue
		}
		parents = append(parents, parentMsg(ts, "プレー", 1))
		api.repliesPages[ts] = [][]slack.Message{{
			parentMsg(ts, "プレー", 1), replyMsg(ts+"1", "U1", "反省"),
		}}
	}
	wantPlays := 6

	type call struct{ done, total int }
	calls := []call{}
	bot := Bot{SlackAPI: api}
	if _, err := bot.expandThreads(testJob(), parents, func(done, total int) {
		calls = append(calls, call{done, total})
	}); err != nil {
		t.Fatalf("expandThreads: %v", err)
	}

	if len(calls) == 0 {
		t.Fatal("進捗コールバックが一度も呼ばれていない")
	}
	for _, c := range calls {
		if c.total != wantPlays {
			t.Fatalf("total = %d, want %d（分母はプレー数）", c.total, wantPlays)
		}
		if c.done > c.total {
			t.Fatalf("done = %d が total = %d を超えている（見出しを数えている）", c.done, c.total)
		}
	}
	// 12 親のうちプレーは 6 件なので、5 件ごとの発火は done=5 の 1 回だけ。
	if last := calls[len(calls)-1]; last.done != focusProgressInterval {
		t.Fatalf("最後の done = %d, want %d", last.done, focusProgressInterval)
	}
}
