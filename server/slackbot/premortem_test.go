package slackbot

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
)

// premortemDigestJSON は fixture をそのまま LLM の応答（Structured Outputs）に見立てる。
func premortemDigestJSON(t *testing.T) string {
	t.Helper()
	b, err := json.Marshal(premortemRiskFixture())
	if err != nil {
		t.Fatalf("marshal digest: %v", err)
	}
	return string(b)
}

// AC-2 / AC-35: 期間の解釈は focus と同じ規則。`<#C…>` は収集チャンネルとして拾う。
func TestNewPremortemJob(t *testing.T) {
	now := time.Date(2026, 9, 10, 15, 0, 0, 0, server.ServiceLocation)

	cases := []struct {
		name        string
		args        []string
		threadTS    string
		wantSources string
		wantThread  bool
		wantOldest  time.Time
		wantErr     bool
	}{
		{
			name: "引数なしは既定日数", args: nil,
			wantSources: "C1", wantOldest: startOfDay(now.AddDate(0, 0, -focusDefaultDays)),
		},
		{
			name: "Nd 指定", args: []string{"20d"},
			wantSources: "C1", wantOldest: startOfDay(now.AddDate(0, 0, -20)),
		},
		{
			name: "M/D 指定", args: []string{"9/1"},
			wantSources: "C1",
			wantOldest:  time.Date(2026, 9, 1, 0, 0, 0, 0, server.ServiceLocation),
		},
		{
			name: "スレッド内で引数なしはスレッド単体", args: nil, threadTS: "900.000000",
			wantSources: "C1", wantThread: true,
		},
		{
			name: "チャンネル指定を拾う", args: []string{"12d", "<#C0ABCDEF|scouting>"},
			wantSources: "C1,C0ABCDEF", wantOldest: startOfDay(now.AddDate(0, 0, -12)),
		},
		{
			name: "チャンネル指定だけでも期間は既定にフォールバック", args: []string{"<#C0ABCDEF>"},
			wantSources: "C1,C0ABCDEF", wantOldest: startOfDay(now.AddDate(0, 0, -focusDefaultDays)),
		},
		{
			name: "スレッド内でもチャンネル指定があれば期間指定として扱う",
			args: []string{"<#C0ABCDEF>"}, threadTS: "900.000000",
			wantSources: "C1,C0ABCDEF", wantOldest: startOfDay(now.AddDate(0, 0, -focusDefaultDays)),
		},
		{
			name: "同じチャンネルを重ねても増えない", args: []string{"<#C1>", "<#C1>"},
			wantSources: "C1", wantOldest: startOfDay(now.AddDate(0, 0, -focusDefaultDays)),
		},
		{name: "読めない期間はエラー", args: []string{"きのう"}, wantErr: true},
		{
			name: "チャンネルが多すぎるとエラー",
			args: []string{"<#C2>", "<#C3>", "<#C4>", "<#C5>", "<#C6>"}, wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job, err := newPremortemJob(c.args, now, "C1", testMentionTS, c.threadTS)
			if c.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("newPremortemJob: %v", err)
			}
			if got := strings.Join(job.Sources, ","); got != c.wantSources {
				t.Fatalf("Sources = %q, want %q", got, c.wantSources)
			}
			if job.ThreadOnly != c.wantThread {
				t.Fatalf("ThreadOnly = %v, want %v", job.ThreadOnly, c.wantThread)
			}
			if c.wantThread {
				return
			}
			if job.Oldest != c.wantOldest.Unix() {
				t.Fatalf("Oldest = %v, want %v",
					time.Unix(job.Oldest, 0).In(server.ServiceLocation), c.wantOldest)
			}
		})
	}
}

// AC-3: task 名は premortem- prefix + ts の `.` を `-` に置換（focus とキューを共用するため）。
func TestPremortemTaskName(t *testing.T) {
	job := premortemJob{Channel: "C1", MentionTS: "1725.123456"}
	if got := premortemTaskName(job); got != "premortem-C1-1725-123456" {
		t.Fatalf("premortemTaskName = %q", got)
	}
	// focus のタスクと衝突しない
	if premortemTaskName(job) == focusTaskName(focusJob{Channel: "C1", MentionTS: "1725.123456"}) {
		t.Fatal("focus と task 名が衝突している")
	}
}

// AC-1 / AC-34: premortem と passion の両方が受付処理に入り、👀 が付く。alias は完全に同じ job。
func TestOnMentionPremortem_Alias(t *testing.T) {
	for _, token := range []string{"premortem", "passion"} {
		t.Run(token, func(t *testing.T) {
			api := newFakeSlackAPI()
			enq := newFakeEnqueuer()
			bot := Bot{SlackAPI: api, Enqueuer: enq}

			bot.onMention(mentionPayload("<@BOT> "+token+" 12d", "C1", testMentionTS, ""))

			if joined(api.added) != focusReactionWorking {
				t.Fatalf("👀 が付いていない: %v", api.added)
			}
			if enq.count() != 1 {
				t.Fatalf("enqueue = %d, want 1", enq.count())
			}
			if enq.uris[0] != PremortemTaskURI {
				t.Fatalf("uri = %q, want %q", enq.uris[0], PremortemTaskURI)
			}
			if !strings.HasPrefix(enq.names[0], "premortem-") {
				t.Fatalf("task 名 = %q", enq.names[0])
			}
			job := premortemJob{}
			if err := json.Unmarshal([]byte(enq.bodies[0]), &job); err != nil {
				t.Fatalf("payload: %v", err)
			}
			if job.Channel != "C1" || len(job.Sources) != 1 || job.Sources[0] != "C1" {
				t.Fatalf("job = %+v", job)
			}
		})
	}
}

// AC-4: premortem は focus と同じ Enqueuer（= 同じキュー）に積む。キューは増えない。
func TestPremortem_SharesFocusQueue(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	bot := Bot{SlackAPI: api, Enqueuer: enq}

	bot.onMention(mentionPayload("<@BOT> focus 12d", "C1", testMentionTS, ""))
	bot.onMention(mentionPayload("<@BOT> premortem 12d", "C1", "351.000000", ""))

	if enq.count() != 2 {
		t.Fatalf("enqueue = %d, want 2（同じ Enqueuer に積む）", enq.count())
	}
	if enq.uris[0] != FocusTaskURI || enq.uris[1] != PremortemTaskURI {
		t.Fatalf("uris = %v", enq.uris)
	}
}

// AC-5 / AC-25 / AC-29: schema が strict で、plays は 3 フィールドだけ、kind は enum。
func TestPremortemReportSchema(t *testing.T) {
	assertStrict := func(t *testing.T, node map[string]any, path string) {
		t.Helper()
		if node["additionalProperties"] != false {
			t.Fatalf("%s: additionalProperties が false でない", path)
		}
		props, _ := node["properties"].(map[string]any)
		required, _ := node["required"].([]any)
		if len(props) != len(required) {
			t.Fatalf("%s: required(%d) と properties(%d) の数が違う", path, len(required), len(props))
		}
		for _, r := range required {
			if _, ok := props[r.(string)]; !ok {
				t.Fatalf("%s: required の %q が properties に無い", path, r)
			}
		}
	}

	assertStrict(t, premortemReportSchema, "root")
	props := premortemReportSchema["properties"].(map[string]any)

	risk := props["risks"].(map[string]any)["items"].(map[string]any)
	assertStrict(t, risk, "risks[]")
	riskProps := risk["properties"].(map[string]any)
	for _, key := range []string{"key", "kind", "title", "label", "scenario", "phase", "unit", "signal", "prevent", "positions", "quote"} {
		if _, ok := riskProps[key]; !ok {
			t.Fatalf("risks[] に %q が無い", key)
		}
	}
	kindEnum := riskProps["kind"].(map[string]any)["enum"].([]any)
	if len(kindEnum) != len(premortemKinds) {
		t.Fatalf("kind の enum = %v, want %v", kindEnum, premortemKinds)
	}

	play := props["plays"].(map[string]any)["items"].(map[string]any)
	assertStrict(t, play, "plays[]")
	playProps := play["properties"].(map[string]any)
	if len(playProps) != 3 {
		t.Fatalf("plays[] のフィールド = %v, want name / risk_keys / positions の 3 つだけ", playProps)
	}
	for _, dead := range []string{"headline", "evidence", "issue"} {
		if _, ok := playProps[dead]; ok {
			t.Fatalf("plays[] に使われない %q が残っている", dead)
		}
	}

	// 応答が premortemDigest に unmarshal できる
	digest := premortemDigest{}
	if err := json.Unmarshal([]byte(premortemDigestJSON(t)), &digest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if digest.Risks[0].Signal == "" || len(digest.Risks[0].Prevent) == 0 {
		t.Fatalf("signal / prevent が読めていない: %+v", digest.Risks[0])
	}
}

// AC-22 / AC-23: 成功時は 👀 を外して ✅ を付け、fallback text 付きで投稿する。
func TestPremortem_EndToEnd(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), premortemTestJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	if len(gpt.requests) != 1 {
		t.Fatalf("ChatGPT calls = %d, want 1", len(gpt.requests))
	}
	if gpt.requests[0].Schema == nil || gpt.requests[0].Schema.Name != premortemReportSchemaName {
		t.Fatalf("Structured Outputs で受けていない: %+v", gpt.requests[0].Schema)
	}
	if len(api.posted) < 2 {
		t.Fatalf("posted = %d, want 受付メッセージ + 1 通目", len(api.posted))
	}
	if !strings.Contains(api.posted[0].Text(), "負け筋を洗い出しています") &&
		!strings.Contains(api.posted[0].Text(), "チャンネルを読んでいます") {
		t.Fatalf("受付メッセージが期待どおりでない: %q", api.posted[0].Text())
	}
	for i, m := range api.posted[1:] {
		if strings.TrimSpace(m.Text()) == "" {
			t.Fatalf("posted[%d] の fallback text が空", i+1)
		}
	}
	if joined(api.added) != focusReactionDone || joined(api.removed) != focusReactionWorking {
		t.Fatalf("リアクションの遷移が期待どおりでない: added=%v removed=%v", api.added, api.removed)
	}
	done := api.updated[len(api.updated)-1].Text()
	if !strings.HasPrefix(done, "✅ ") {
		t.Fatalf("完了メッセージ = %q", done)
	}
}

// AC-23: 失敗時は黙らず、理由をスレッドに返して 👀 を外す。
func TestPremortem_ReportsError(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{err: errors.New("boom")}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), premortemTestJob()); err == nil {
		t.Fatal("err = nil, want error")
	}
	last := api.posted[len(api.posted)-1]
	if !strings.Contains(last.Text(), "boom") {
		t.Fatalf("失敗理由がスレッドに返っていない: %q", last.Text())
	}
	if joined(api.removed) != focusReactionWorking || len(api.added) != 0 {
		t.Fatalf("リアクション: added=%v removed=%v", api.added, api.removed)
	}
}

// AC-20: 構造化に失敗しても落とさず、LLM の生出力を平文で投稿する。
func TestPremortem_PlainTextFallback(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: "これは JSON ではありません"}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), premortemTestJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	body := api.posted[len(api.posted)-1].Text()
	if !strings.Contains(body, "これは JSON ではありません") {
		t.Fatalf("平文フォールバックが投稿されていない: %q", body)
	}
	if len(api.posted[len(api.posted)-1].Blocks()) != 0 {
		t.Fatal("平文フォールバックなのに blocks が付いている")
	}
}

// AC-20: risks が 0 件でも平文フォールバックに倒す（1 通目が見出しだけになるのを防ぐ）。
func TestPremortem_NoRisksFallsBackToPlainText(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: `{"risks":[],"plays":[]}`}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), premortemTestJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	if len(api.posted[len(api.posted)-1].Blocks()) != 0 {
		t.Fatal("risks 0 件なのに blocks を組んでいる")
	}
}

// AC-35 / AC-36: 複数チャンネルを収集し、読めなかったチャンネルは握り潰さず明示する。
func TestPremortem_MultiChannel(t *testing.T) {
	api := newFakeSlackAPI()
	api.historyByChannel = map[string]*slack.GetConversationHistoryResponse{
		"C1": historyPage("", parentMsg("100.000000", "プレーA", 1)),
		"C2": historyPage("", parentMsg("200.000000", "スカウティング資料", 1)),
	}
	api.repliesPages["100.000000"] = [][]slack.Message{{
		parentMsg("100.000000", "プレーA", 1), replyMsg("101.000000", "U1", "反省1"),
	}}
	api.repliesPages["200.000000"] = [][]slack.Message{{
		parentMsg("200.000000", "スカウティング資料", 1), replyMsg("201.000000", "U2", "相手はパス偏重"),
	}}
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	job := premortemJob{Channel: "C1", Sources: []string{"C1", "C2"}, MentionTS: testMentionTS}
	if err := bot.premortem(t.Context(), job, nil); err != nil {
		t.Fatalf("premortem: %v", err)
	}

	prompt := gpt.requests[0].User
	if !strings.Contains(prompt, "プレーA") || !strings.Contains(prompt, "スカウティング資料") {
		t.Fatalf("両方のチャンネルが収集されていない:\n%s", prompt)
	}
	channels := map[string]bool{}
	for _, p := range api.historyParams {
		channels[p.ChannelID] = true
	}
	if !channels["C1"] || !channels["C2"] {
		t.Fatalf("history を引いたチャンネル = %v", channels)
	}
}

// AC-36: 1 チャンネルが読めなくても他は続行し、読めなかったチャンネルをスレッドに明示する。
func TestPremortem_UnreadableChannelIsReported(t *testing.T) {
	api := newFakeSlackAPI()
	api.historyByChannel = map[string]*slack.GetConversationHistoryResponse{
		"C1": historyPage("", parentMsg("100.000000", "プレーA", 1)),
	}
	api.historyErrByChannel = map[string]error{"C2": errors.New("not_in_channel")}
	api.repliesPages["100.000000"] = [][]slack.Message{{
		parentMsg("100.000000", "プレーA", 1), replyMsg("101.000000", "U1", "反省1"),
	}}
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	job := premortemJob{Channel: "C1", Sources: []string{"C1", "C2"}, MentionTS: testMentionTS}
	if err := bot.premortem(t.Context(), job, nil); err != nil {
		t.Fatalf("読めないチャンネルで全体が止まっている: %v", err)
	}
	notice := ""
	for _, m := range api.posted {
		if strings.Contains(m.Text(), "<#C2>") {
			notice = m.Text()
		}
	}
	if notice == "" {
		t.Fatalf("読めなかったチャンネルが通知されていない: %+v", api.posted)
	}
	if !strings.Contains(gpt.requests[0].User, "プレーA") {
		t.Fatal("読めたチャンネルの収集まで止まっている")
	}
}

// AC-36: 全チャンネルが読めなければエラーにする（黙って「対象なし」にしない）。
func TestPremortem_AllChannelsUnreadable(t *testing.T) {
	api := newFakeSlackAPI()
	api.historyErrByChannel = map[string]error{"C1": errors.New("not_in_channel")}
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{}}

	job := premortemJob{Channel: "C1", Sources: []string{"C1"}, MentionTS: testMentionTS}
	if err := bot.premortem(t.Context(), job, nil); err == nil {
		t.Fatal("err = nil, want error")
	}
}

// AC-37: リンクの unfurl とテキスト系ファイルの中身がプロンプトに載る。
func TestRenderPremortemThreads_Materials(t *testing.T) {
	parent := parentMsg("100.000000", "相手チームの資料", 1)
	parent.Attachments = []slack.Attachment{{
		Title: "シルバースター 第 3 節ハイライト", Text: "パス 32 / ラン 11。第 4Q は\nラン中心。",
	}}
	reply := replyMsg("101.000000", "U1", "傾向まとめました")
	reply.Files = []slack.File{
		{Title: "scouting.md", Filetype: "markdown", Preview: "1st down はラン 6 割"},
		{Title: "whiteboard.png", Filetype: "png"},
	}

	got := renderPremortemThreads([]playThread{{Parent: parent, Replies: []slack.Message{reply}}}, nil)

	for _, want := range []string{
		"[リンク] シルバースター 第 3 節ハイライト — パス 32 / ラン 11。第 4Q は ラン中心。",
		"[ファイル] scouting.md: 1st down はラン 6 割",
		"[ファイル] whiteboard.png（png。本文は取得できません）",
		"- U1: 傾向まとめました",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("プロンプトに %q が無い:\n%s", want, got)
		}
	}
}

// AC-26: 入力が分割されたら、その旨をスレッドに 1 行出す（黙って質を落とさない）。
func TestPremortem_SplitNotice(t *testing.T) {
	long := strings.Repeat("あ", focusPromptRuneBudget/2+100)
	api := newFakeSlackAPI()
	api.historyByChannel = map[string]*slack.GetConversationHistoryResponse{
		"C1": historyPage("",
			parentMsg("100.000000", long, 1),
			parentMsg("200.000000", long, 1),
			parentMsg("300.000000", long, 1),
		),
	}
	for _, ts := range []string{"100.000000", "200.000000", "300.000000"} {
		api.repliesPages[ts] = [][]slack.Message{{
			parentMsg(ts, long, 1), replyMsg(ts+"1", "U1", "反省"),
		}}
	}
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	job := premortemJob{Channel: "C1", Sources: []string{"C1"}, MentionTS: testMentionTS}
	if err := bot.premortem(t.Context(), job, nil); err != nil {
		t.Fatalf("premortem: %v", err)
	}
	if len(gpt.requests) < 2 {
		t.Fatalf("分割されていない（LLM 呼び出し %d 回）", len(gpt.requests))
	}
	notice := false
	for _, m := range api.posted {
		if strings.Contains(m.Text(), "入力を") && strings.Contains(m.Text(), "分割して読みました") {
			notice = true
		}
	}
	if !notice {
		t.Fatalf("分割の通知が出ていない: %+v", api.posted)
	}
}

// premortemFewTargets は focus と同じ閾値。スレッド単体は常に few。
func TestPremortemFewTargets(t *testing.T) {
	if !premortemFewTargets(premortemJob{ThreadOnly: true}, playThreads(50)) {
		t.Fatal("スレッド単体が few になっていない")
	}
	if premortemFewTargets(premortemJob{}, playThreads(focusFewTargetsThreshold)) {
		t.Fatal("閾値ちょうどで few になっている")
	}
	if !premortemFewTargets(premortemJob{}, playThreads(focusFewTargetsThreshold-1)) {
		t.Fatal("閾値未満で few になっていない")
	}
}
