package slackbot

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/triax/hub/server"
)

const testSlashToken = "verification-token"

// slashForm は Slack が送ってくる application/x-www-form-urlencoded のペイロード。
func slashForm(command, text string) url.Values {
	return url.Values{
		"token":      {testSlashToken},
		"command":    {command},
		"text":       {text},
		"channel_id": {"C1"},
		"user_id":    {"U9"},
	}
}

// postSlash はハンドラを直接叩き、レスポンスを返す。
func postSlash(bot Bot, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/slack/slashcommands", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	bot.SlashCommands(rec, req)
	return rec
}

// responseURLCatcher は response_url への投稿を捕まえる（postSlackJSON は実 HTTP を打つ）。
func responseURLCatcher(t *testing.T) (string, func() []string) {
	t.Helper()
	got := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := map[string]string{}
		_ = json.Unmarshal(body, &payload)
		got = append(got, payload["text"])
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []string { return got }
}

func slashBot(api *fakeSlackAPI, enq *fakeEnqueuer) Bot {
	return Bot{VerificationToken: testSlashToken, SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq}
}

// AC-1 / AC-2 / AC-10: /premortem はチャンネルに何も残さず、受付を ephemeral で出して
// 打った人だけに見える job を積む。
func TestSlash_Premortem(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	form := slashForm("/premortem", "12d")
	form.Set("trigger_id", "13345224609.738474920.8088930838d88f008e0")
	rec := postSlash(slashBot(api, enq), form)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("body = %q, want 空", body)
	}
	// AC-1: チャンネルへの通常投稿はゼロ（アンカーを廃止した）
	if len(api.posted) != 0 {
		t.Fatalf("チャンネルに投稿している: %+v", api.posted)
	}
	// AC-2: 受付は ephemeral で、宛先は打った人
	if len(api.ephemeral) != 1 {
		t.Fatalf("ephemeral = %d, want 受付 1 通", len(api.ephemeral))
	}
	receipt := api.ephemeral[0]
	if receipt.Channel != "C1" || receipt.Timestamp != "U9" {
		t.Fatalf("受付の宛先 = ch:%q user:%q, want C1/U9", receipt.Channel, receipt.Timestamp)
	}
	if !strings.Contains(receipt.Text(), "あなただけに見えます") {
		t.Fatalf("受付メッセージ = %q", receipt.Text())
	}
	// AC-5: リアクションを付ける先が無い
	if len(api.added) != 0 || len(api.removed) != 0 {
		t.Fatalf("リアクションを操作している: added=%v removed=%v", api.added, api.removed)
	}

	if enq.count() != 1 || enq.uris[0] != PremortemTaskURI {
		t.Fatalf("enqueue = %d / uri = %v", enq.count(), enq.uris)
	}
	job := premortemJob{}
	if err := json.Unmarshal([]byte(enq.bodies[0]), &job); err != nil {
		t.Fatalf("payload: %v", err)
	}
	// AC-10
	if !job.Ephemeral || job.UserID != "U9" || job.MentionTS != "" {
		t.Fatalf("job = %+v, want Ephemeral=true UserID=U9 MentionTS=空", job)
	}
	// AC-9: task 名は trigger_id 由来
	if !strings.Contains(enq.names[0], "13345224609-738474920") {
		t.Fatalf("task 名 = %q, want trigger_id 由来", enq.names[0])
	}
}

// AC-9: 同じ人が連続で打っても trigger_id が違えば task 名が衝突しない。
func TestSlash_TaskNameFromTriggerID(t *testing.T) {
	a := premortemTaskName(premortemJob{Channel: "C1", TaskKey: "111.222.aaa"})
	b := premortemTaskName(premortemJob{Channel: "C1", TaskKey: "111.333.bbb"})
	if a == b {
		t.Fatalf("task 名が衝突している: %q", a)
	}
	if strings.Contains(a, ".") {
		t.Fatalf("task ID に `.` が残っている: %q", a)
	}
	// mention 経由（TaskKey = MentionTS）とも衝突しない
	if a == premortemTaskName(premortemJob{Channel: "C1", TaskKey: testMentionTS}) {
		t.Fatal("mention の task 名と衝突している")
	}
}

// AC-8: 受付 ephemeral が失敗（bot 未参加）→ response_url にエラーを返し enqueue しない。
func TestSlash_ReceiptFailure(t *testing.T) {
	responseURL, texts := responseURLCatcher(t)
	api := newFakeSlackAPI()
	api.postErr = errors.New("not_in_channel")
	enq := newFakeEnqueuer()

	form := slashForm("/premortem", "12d")
	form.Set("response_url", responseURL)
	rec := postSlash(slashBot(api, enq), form)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if enq.count() != 0 {
		t.Fatalf("受付に失敗したのに enqueue している: %d", enq.count())
	}
	if len(texts()) != 1 || !strings.Contains(texts()[0], "bot が参加しているか") {
		t.Fatalf("response_url へのエラーが期待どおりでない: %v", texts())
	}
}

// AC-14: 期間の指定が読めないときは response_url で返す（チャンネルには何も出さない）。
func TestSlash_InvalidPeriodIsReported(t *testing.T) {
	responseURL, texts := responseURLCatcher(t)
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()

	form := slashForm("/premortem", "きのう")
	form.Set("response_url", responseURL)
	postSlash(slashBot(api, enq), form)

	if enq.count() != 0 {
		t.Fatalf("enqueue = %d, want 0", enq.count())
	}
	if len(api.posted) != 0 || len(api.ephemeral) != 0 {
		t.Fatalf("チャンネルに何か出ている: posted=%v ephemeral=%v", api.posted, api.ephemeral)
	}
	if len(texts()) != 1 || !strings.Contains(texts()[0], "期間の指定") {
		t.Fatalf("エラーが返っていない: %v", texts())
	}
}

// AC-6: text の解釈が mention と同じ。**mention 経由で同じ引数を流した実 job** と
// 突き合わせる（slash と同じトークナイザで期待値を作ると自己循環になるため）。
func TestSlash_ParsesTextLikeMention(t *testing.T) {
	const args = "20d <#C0ABCDEF|scouting>"

	slashAPI, slashEnq := newFakeSlackAPI(), newFakeEnqueuer()
	postSlash(slashBot(slashAPI, slashEnq), slashForm("/premortem", args))
	slashJob := premortemJob{}
	if err := json.Unmarshal([]byte(slashEnq.bodies[0]), &slashJob); err != nil {
		t.Fatalf("slash payload: %v", err)
	}

	mentionAPI, mentionEnq := newFakeSlackAPI(), newFakeEnqueuer()
	mentionBot := Bot{SlackAPI: mentionAPI, ChatGPT: &fakeChatGPT{}, Enqueuer: mentionEnq}
	mentionBot.onMention(mentionPayload("<@BOT> premortem "+args, "C1", testMentionTS, ""))
	mentionJob := premortemJob{}
	if err := json.Unmarshal([]byte(mentionEnq.bodies[0]), &mentionJob); err != nil {
		t.Fatalf("mention payload: %v", err)
	}

	if strings.Join(slashJob.Sources, ",") != strings.Join(mentionJob.Sources, ",") {
		t.Fatalf("Sources = %v, want %v", slashJob.Sources, mentionJob.Sources)
	}
	if slashJob.Oldest != mentionJob.Oldest {
		t.Fatalf("Oldest = %v, want %v",
			time.Unix(slashJob.Oldest, 0).In(server.ServiceLocation),
			time.Unix(mentionJob.Oldest, 0).In(server.ServiceLocation))
	}
	if slashJob.ThreadOnly != mentionJob.ThreadOnly {
		t.Fatalf("ThreadOnly = %v, want %v", slashJob.ThreadOnly, mentionJob.ThreadOnly)
	}
	// 実際にチャンネル指定と期間が効いていること（両方が同じように壊れていても気づけるように）
	if len(slashJob.Sources) != 2 || slashJob.Sources[1] != "C0ABCDEF" {
		t.Fatalf("チャンネル指定が拾えていない: %v", slashJob.Sources)
	}
	if want := startOfDay(time.Now().In(server.ServiceLocation).AddDate(0, 0, -20)).Unix(); slashJob.Oldest != want {
		t.Fatalf("Oldest = %d, want %d（20d）", slashJob.Oldest, want)
	}
}

// AC-5: アンカーは bot 投稿なので、収集時にプレーとして数えられない。
func TestSlash_AnchorIsExcludedFromPlays(t *testing.T) {
	anchor := botMsg("900.000001", "🧨 <@U9> が premortem を実行します")
	if !isBotOrSystem(anchor, "") {
		t.Fatal("bot 投稿のアンカーが除外されていない")
	}
	if !isSkippableParent(anchor, "") {
		t.Fatal("アンカーが親候補から除外されていない")
	}
}

// AC-3: 未知の command は従来の「ありがとう」処理へ落ちる。
func TestSlash_FallsBackToThankYou(t *testing.T) {
	responseURL, texts := responseURLCatcher(t)
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()

	form := slashForm("/thankyou", "<@U123> 助かりました")
	form.Set("response_url", responseURL)
	postSlash(slashBot(api, enq), form)

	if enq.count() != 0 {
		t.Fatalf("premortem を enqueue している: %d", enq.count())
	}
	if len(api.posted) == 0 {
		t.Fatal("DM が送られていない（従来の挙動が壊れている）")
	}
	if got := api.posted[0]; got.Channel != "D1" || !strings.Contains(got.Text(), "助かりました") {
		t.Fatalf("DM が期待どおりでない: %+v", got)
	}
	if len(texts()) == 0 || !strings.Contains(texts()[0], "に伝えました") {
		t.Fatalf("response_url へのフィードバックが無い: %v", texts())
	}
}

// AC-8 / AC-10: token 不一致は 4xx で、Slack API を 1 回も叩かない。
func TestSlash_RejectsBadToken(t *testing.T) {
	for _, command := range []string{"/premortem", "/thankyou"} {
		t.Run(command, func(t *testing.T) {
			api := newFakeSlackAPI()
			enq := newFakeEnqueuer()
			form := slashForm(command, "12d <@U123>")
			form.Set("token", "wrong")

			rec := postSlash(slashBot(api, enq), form)

			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("status = %d, want 4xx", rec.Code)
			}
			if len(api.posted) != 0 || len(api.added) != 0 {
				t.Fatalf("Slack API を叩いている: posted=%v added=%v", api.posted, api.added)
			}
			if enq.count() != 0 {
				t.Fatalf("enqueue = %d, want 0", enq.count())
			}
		})
	}
}

// AC-9: VerificationToken 未設定（ローカル開発）では検証を素通しする。
func TestSlash_SkipsVerificationWhenUnset(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{}, Enqueuer: enq} // VerificationToken なし

	form := slashForm("/premortem", "12d")
	form.Set("token", "なんでもよい")
	if rec := postSlash(bot, form); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if enq.count() != 1 {
		t.Fatalf("enqueue = %d, want 1", enq.count())
	}
}

// AC-11: ハンドラ内で収集も LLM 呼び出しもしない（重い処理は enqueue の先）。
func TestSlash_DoesNoHeavyWorkInHandler(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	gpt := &fakeChatGPT{}
	bot := Bot{VerificationToken: testSlashToken, SlackAPI: api, ChatGPT: gpt, Enqueuer: enq}

	postSlash(bot, slashForm("/premortem", "12d"))

	if len(gpt.requests) != 0 {
		t.Fatalf("ハンドラ内で LLM を呼んでいる: %d 回", len(gpt.requests))
	}
	if api.historyCalls != 0 || len(api.historyParams) != 0 {
		t.Fatalf("ハンドラ内で収集している: %d", len(api.historyParams))
	}
}

// AC-13: アンカー投稿が失敗したら response_url にエラーを返し、enqueue しない。
func TestSlash_AnchorFailure(t *testing.T) {
	responseURL, texts := responseURLCatcher(t)
	api := newFakeSlackAPI()
	api.postErr = errors.New("not_in_channel")
	enq := newFakeEnqueuer()

	form := slashForm("/premortem", "12d")
	form.Set("response_url", responseURL)
	rec := postSlash(slashBot(api, enq), form)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200（Slack にはエラーを返さない）", rec.Code)
	}
	if enq.count() != 0 {
		t.Fatalf("アンカーが無いのに enqueue している: %d", enq.count())
	}
	if len(texts()) != 1 || !strings.Contains(texts()[0], "bot が参加しているか") {
		t.Fatalf("response_url へのエラーが期待どおりでない: %v", texts())
	}
}

// ---- ephemeral 配送の end-to-end ---------------------------------------------

func ephemeralJob() premortemJob {
	return premortemJob{
		Channel: "C1", Sources: []string{"C1"}, UserID: "U9",
		Ephemeral: true, TaskKey: "111.222.aaa",
	}
}

// AC-1 / AC-2 / AC-5 / AC-6: ワーカーはチャンネルに何も出さず、結果を打った人だけに届ける。
// 編集もリアクションも起きない。
//
// 受付は slash ハンドラ側で出す（疎通確認を兼ねるため）ので、ワーカーは出さない。
// ユーザから見える ephemeral は「受付（ハンドラ）+ 結果（ワーカー）」の 2 通で、
// 受付側は TestSlash_Premortem が押さえている（AC-6b）。
func TestPremortem_EphemeralEndToEnd(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	job := ephemeralJob()
	if err := bot.runPremortem(t.Context(), job); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}

	// AC-1: チャンネルへの通常投稿はゼロ
	if len(api.posted) != 0 {
		t.Fatalf("チャンネルに投稿している: %+v", api.posted)
	}
	// AC-6: 編集も起きない（受付を完了に差し替えない）
	if len(api.updated) != 0 {
		t.Fatalf("UpdateMessage を呼んでいる: %+v", api.updated)
	}
	// AC-5: リアクションも無い
	if len(api.added) != 0 || len(api.removed) != 0 {
		t.Fatalf("リアクションを操作している: added=%v removed=%v", api.added, api.removed)
	}
	// 完了メタの 3 通目を出さない（ワーカーが出すのは結果 1 通だけ）
	if len(api.ephemeral) != 1 {
		t.Fatalf("ephemeral = %d 通, want 1（結果のみ。受付はハンドラ側）", len(api.ephemeral))
	}
	for i, m := range api.ephemeral {
		if m.Channel != "C1" || m.Timestamp != "U9" {
			t.Fatalf("ephemeral[%d] の宛先 = ch:%q user:%q", i, m.Channel, m.Timestamp)
		}
		if strings.TrimSpace(m.Text()) == "" {
			t.Fatalf("ephemeral[%d] の fallback text が空", i)
		}
	}
	if len(api.ephemeral[0].Blocks()) == 0 {
		t.Fatal("結果に blocks が載っていない")
	}
}

// AC-3 / AC-4: 結果の blocks は mention 経由と同じ構成で、末尾の案内だけが違う。
func TestPremortem_EphemeralBlocksMatchMention(t *testing.T) {
	report := rankRisks(premortemRiskFixture(), false)
	now := time.Now()
	threads := playThreads(8)

	channelJob := premortemTestJob()
	ephJob := channelJob
	ephJob.Ephemeral = true
	ephJob.UserID = "U9"

	channelBlocks := premortemDigestBlocks(channelJob, threads, "9/21(日) vs A", now, report)
	ephBlocks := premortemDigestBlocks(ephJob, threads, "9/21(日) vs A", now, report)

	// AC-3: block の並びは同一
	if got, want := strings.Join(blockTypes(ephBlocks), ","), strings.Join(blockTypes(channelBlocks), ","); got != want {
		t.Fatalf("block 構成が違う:\n eph=%s\n ch =%s", got, want)
	}
	if len(ephBlocks) != len(channelBlocks) {
		t.Fatalf("block 数が違う: %d vs %d", len(ephBlocks), len(channelBlocks))
	}

	// AC-4: 案内文だけが違う
	chBody, ephBody := blocksJSON(t, channelBlocks), blocksJSON(t, ephBlocks)
	if !strings.Contains(chBody, "反論・追加はこのスレッドへ") {
		t.Fatal("mention 側の案内が変わっている")
	}
	if strings.Contains(ephBody, "このスレッドへ") {
		t.Fatal("ephemeral なのにスレッドへの導線が残っている")
	}
	if !strings.Contains(ephBody, "あなただけに見えています") {
		t.Fatalf("ephemeral の案内が期待どおりでない: %s", ephBody[len(ephBody)-400:])
	}
	// 案内以外は一致する（案内の block を落として比べる）
	if trim := func(s string) string { return s[:strings.LastIndex(s, "premortem_guide")] }; trim(chBody) != trim(ephBody) {
		t.Fatal("案内以外の中身が違う")
	}
}

// AC-7: mention の挙動が一切変わらない（sink 抽象化の回帰よけ）。
func TestPremortem_MentionUnchanged(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), premortemTestJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	if len(api.ephemeral) != 0 {
		t.Fatalf("mention なのに ephemeral を使っている: %+v", api.ephemeral)
	}
	if len(api.posted) < 2 {
		t.Fatalf("posted = %d, want 受付 + 結果", len(api.posted))
	}
	// 受付はスレッド返信、結果の 1 通目は broadcast
	if api.posted[0].ThreadTS() != testMentionTS {
		t.Fatalf("受付がスレッドに出ていない: %+v", api.posted[0])
	}
	if api.posted[1].Broadcast() == "" {
		t.Fatalf("1 通目が broadcast されていない: %+v", api.posted[1])
	}
	// 完了メタへの差し替えとリアクション遷移
	if len(api.updated) == 0 || !strings.HasPrefix(api.updated[len(api.updated)-1].Text(), "✅ ") {
		t.Fatalf("完了メタに差し替わっていない: %+v", api.updated)
	}
	if joined(api.added) != focusReactionDone || joined(api.removed) != focusReactionWorking {
		t.Fatalf("リアクションの遷移が変わっている: added=%v removed=%v", api.added, api.removed)
	}
}

// AC-11: MentionTS が空でも収集が壊れない（除外すべきアンカーが存在しないだけ）。
func TestPremortem_EphemeralCollects(t *testing.T) {
	api := focusFixture()
	gpt := &fakeChatGPT{reply: premortemDigestJSON(t)}
	bot := Bot{SlackAPI: api, ChatGPT: gpt}

	if err := bot.runPremortem(t.Context(), ephemeralJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	prompt := gpt.requests[0].User
	for _, want := range []string{"プレーA", "プレーB", "反省1"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("収集できていない（%q が無い）:\n%s", want, prompt)
		}
	}
}

// AC-12: 平文フォールバックも ephemeral で届く。
func TestPremortem_EphemeralPlainTextFallback(t *testing.T) {
	api := focusFixture()
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{reply: "これは JSON ではありません"}}

	if err := bot.runPremortem(t.Context(), ephemeralJob()); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	if len(api.posted) != 0 {
		t.Fatalf("チャンネルに漏れている: %+v", api.posted)
	}
	last := api.ephemeral[len(api.ephemeral)-1]
	if !strings.Contains(last.Text(), "これは JSON ではありません") {
		t.Fatalf("平文フォールバックが届いていない: %q", last.Text())
	}
	if len(last.Blocks()) != 0 {
		t.Fatal("平文フォールバックなのに blocks が付いている")
	}
}

// AC-13: 読めないチャンネルの警告も ephemeral で届く（チャンネルに漏れない）。
func TestPremortem_EphemeralUnreadableNotice(t *testing.T) {
	api := newFakeSlackAPI()
	api.historyByChannel = map[string]*slack.GetConversationHistoryResponse{
		"C1": historyPage("", parentMsg("100.000000", "プレーA", 1)),
	}
	api.historyErrByChannel = map[string]error{"C2": errors.New("not_in_channel")}
	api.repliesPages["100.000000"] = [][]slack.Message{{
		parentMsg("100.000000", "プレーA", 1), replyMsg("101.000000", "U1", "反省1"),
	}}
	bot := Bot{SlackAPI: api, ChatGPT: &fakeChatGPT{reply: premortemDigestJSON(t)}}

	job := ephemeralJob()
	job.Sources = []string{"C1", "C2"}
	if err := bot.runPremortem(t.Context(), job); err != nil {
		t.Fatalf("runPremortem: %v", err)
	}
	if len(api.posted) != 0 {
		t.Fatalf("警告がチャンネルに漏れている: %+v", api.posted)
	}
	notice := ""
	for _, m := range api.ephemeral {
		if strings.Contains(m.Text(), "<#C2>") {
			notice = m.Text()
		}
	}
	if notice == "" {
		t.Fatalf("読めなかったチャンネルの警告が届いていない: %+v", api.ephemeral)
	}
}
