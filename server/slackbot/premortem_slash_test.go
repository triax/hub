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

// AC-1 / AC-4 / AC-12: /premortem がアンカーを 1 通出し、その ts で enqueue する。
func TestSlash_Premortem(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	rec := postSlash(slashBot(api, enq), slashForm("/premortem", "12d"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body != "" {
		t.Fatalf("body = %q, want 空（可視のフィードバックはアンカー投稿が担う）", body)
	}
	// AC-4: アンカー投稿が 1 通
	if len(api.posted) != 1 {
		t.Fatalf("posted = %d, want アンカー 1 通", len(api.posted))
	}
	anchor := api.posted[0]
	if anchor.Channel != "C1" || !strings.Contains(anchor.Text(), "<@U9>") {
		t.Fatalf("アンカー投稿が期待どおりでない: %+v", anchor)
	}
	if anchor.ThreadTS() != "" {
		t.Fatal("アンカーはトップレベルに出す（スレッド返信にしない）")
	}
	// 👀 が付く
	if joined(api.added) != focusReactionWorking {
		t.Fatalf("👀 が付いていない: %v", api.added)
	}
	// AC-1: enqueue
	if enq.count() != 1 {
		t.Fatalf("enqueue = %d, want 1", enq.count())
	}
	if enq.uris[0] != PremortemTaskURI {
		t.Fatalf("uri = %q, want %q", enq.uris[0], PremortemTaskURI)
	}
	job := premortemJob{}
	if err := json.Unmarshal([]byte(enq.bodies[0]), &job); err != nil {
		t.Fatalf("payload: %v", err)
	}
	// AC-4: アンカーの ts が MentionTS になる
	if job.MentionTS != "900.000001" {
		t.Fatalf("MentionTS = %q, want アンカーの ts", job.MentionTS)
	}
	if job.Channel != "C1" || job.ThreadOnly {
		t.Fatalf("job = %+v", job)
	}
}

// AC-2: `/passion` は alias にしない（#691 で廃止）。premortem を起動せず、
// 未知の command として既定（ありがとう）へ落ちる。復活させないための番人。
func TestSlash_PassionIsNotAnAlias(t *testing.T) {
	responseURL, texts := responseURLCatcher(t)
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()

	form := slashForm("/passion", "12d")
	form.Set("response_url", responseURL)
	postSlash(slashBot(api, enq), form)

	if enq.count() != 0 {
		t.Fatalf("/passion で premortem が起動している: %d 件 enqueue", enq.count())
	}
	if len(api.posted) != 0 {
		t.Fatalf("アンカーが投稿されている: %+v", api.posted)
	}
	// 既定（ありがとう）に落ちる。メンションが無いので使い方の案内が返る
	if len(texts()) != 1 || !strings.Contains(texts()[0], "メンションで指定") {
		t.Fatalf("既定の command に落ちていない: %v", texts())
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

// AC-7: task 名が mention 経由と衝突しない（アンカーの ts が違う）。
func TestSlash_TaskNameDiffersFromMention(t *testing.T) {
	slash := premortemTaskName(premortemJob{Channel: "C1", MentionTS: "900.000001"})
	mention := premortemTaskName(premortemJob{Channel: "C1", MentionTS: testMentionTS})
	if slash == mention {
		t.Fatalf("task 名が衝突している: %q", slash)
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

// AC-14: 期間の指定が読めないときは黙らず、アンカーのスレッドに理由を返して 👀 を外す。
func TestSlash_InvalidPeriodIsReported(t *testing.T) {
	api := newFakeSlackAPI()
	enq := newFakeEnqueuer()
	postSlash(slashBot(api, enq), slashForm("/premortem", "きのう"))

	if enq.count() != 0 {
		t.Fatalf("enqueue = %d, want 0", enq.count())
	}
	if len(api.posted) != 2 {
		t.Fatalf("posted = %d, want アンカー + エラー返信", len(api.posted))
	}
	if !strings.Contains(api.posted[1].Text(), "期間の指定") {
		t.Fatalf("エラーが返っていない: %q", api.posted[1].Text())
	}
	if joined(api.removed) != focusReactionWorking {
		t.Fatalf("👀 が外れていない: %v", api.removed)
	}
}
