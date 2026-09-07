package slackbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

const (
	// FocusTaskURI は Cloud Tasks から叩かれるワーカーの相対パス。
	FocusTaskURI = "/tasks/focus"

	focusDefaultDays      = 12   // 期間指定が無いときに遡る日数（前週・前々週の練習を拾う幅）
	focusHistoryPageLimit = 200  // conversations.history の 1 ページあたり件数
	focusRepliesPageLimit = 200  // conversations.replies の 1 ページあたり件数
	focusChunkSize        = 3500 // Slack の 1 メッセージ上限（実質 4000 文字）に対する安全域
	focusProgressInterval = 5    // 何スレッドごとに進捗メッセージを更新するか
	focusRateLimitRetries = 3    // 429 に対する再試行回数

	// 要約プロンプトに詰め込む上限（rune 数）。トークン数は rune 数 / 2 で概算するので
	// これは約 60,000 トークンに相当する。超えたときだけスレッド単位に分割する。
	focusPromptRuneBudget = 120000

	// 返信の付いた投稿がこれ未満（またはスレッド単体要約）なら focus を 1〜3 点に絞る。
	focusFewTargetsThreshold = 5

	focusReactionWorking = "eyes"
	focusReactionDone    = "white_check_mark"

	focusEmptyMessage = "対象の投稿がありませんでした"
)

var (
	focusMentionPattern = regexp.MustCompile(`<@([A-Za-z0-9]+)>`)
	focusDaysPattern    = regexp.MustCompile(`^(\d+)d$`)
	focusMonthDayPatten = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})$`)
)

// focusJob は Webhook（enqueue 側）とワーカー（/tasks/focus）の間で受け渡す仕事の単位。
type focusJob struct {
	Channel    string `json:"channel"`
	MentionTS  string `json:"mention_ts"`
	ThreadTS   string `json:"thread_ts"`
	Oldest     int64  `json:"oldest"`
	ThreadOnly bool   `json:"thread_only"`
}

// playThread は「1 プレー（または見出し）の親投稿」と、その反省スレッドの返信。
type playThread struct {
	Parent  slack.Message
	Replies []slack.Message
}

// IsHeadline は返信の無い親投稿（`GL Drive1` のような区切り）かどうかを返す。
// 受付メッセージの件数見積り用。プレーか見出しかの判定そのものは要約 LLM に委ねる。
func (t playThread) IsHeadline() bool { return len(t.Replies) == 0 }

// focusDigest は要約 LLM に返させる構造化出力。Focus がチャンネルにも出す
// 「この期間の focus」、Sections がスレッド内に続けるプレー別の詳細。
type focusDigest struct {
	Focus    []focusItem    `json:"focus"`
	Sections []focusSection `json:"sections"`
}

// focusItem は focus 1 点。Count は根拠になったプレー数、Plays は代表プレー名。
type focusItem struct {
	Title     string   `json:"title"`
	Detail    string   `json:"detail"`
	Positions []string `json:"positions"`
	Count     int      `json:"count"`
	// Plays は 1 通目には描かない（3〜5 点を短く保つため）。LLM に「根拠のプレーを挙げろ」と
	// 課すことで Count の裏取りをさせる狙いで受け取っている。
	// stellar:debt(scope) 受け取るだけで描画していない。upgrade: 詳細スレッドで focus と
	// プレーを相互リンクするか、不要なら prompt ごと落とす
	Plays []string `json:"plays"`
}

// focusSection は見出し（ドリルやシリーズの区切り）とその配下のプレー。
type focusSection struct {
	Headline string      `json:"headline"`
	Plays    []focusPlay `json:"plays"`
}

type focusPlay struct {
	Name   string   `json:"name"`
	Points []string `json:"points"`
}

// valid は Block Kit で描けるだけの中身があるかを返す。focus が 0 件のまま
// ordered list を組むと空の rich_text_list になり Slack に invalid_blocks で
// 弾かれるので、その場合は平文フォールバックに倒す。
func (d focusDigest) valid() bool { return len(d.Focus) > 0 }

// focusDigestSchemaName は Structured Outputs に渡す schema 名。
const focusDigestSchemaName = "focus_digest"

// strictObject は Structured Outputs（strict）が要求する形の object schema を組む。
// strict では省略可能なフィールドを作れないため、required は常に properties の
// 全キーになる。ここで導出することで、property を足したときの書き漏れを防ぐ。
func strictObject(properties map[string]any) map[string]any {
	required := make([]any, 0, len(properties))
	for name := range properties {
		required = append(required, name)
	}
	sort.Slice(required, func(i, j int) bool { return required[i].(string) < required[j].(string) })
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// stringField / arrayOf は schema の読みやすさのための小さな組み立て子。
func stringField() map[string]any { return map[string]any{"type": "string"} }

func arrayOf(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

// focusDigestSchema は focusDigest の JSON Schema。focusDigest の json タグと
// 1:1 で対応させる。
var focusDigestSchema = strictObject(map[string]any{
	"focus": arrayOf(strictObject(map[string]any{
		"title":     stringField(),
		"detail":    stringField(),
		"positions": arrayOf(stringField()),
		"count":     map[string]any{"type": "integer"},
		"plays":     arrayOf(stringField()),
	})),
	"sections": arrayOf(strictObject(map[string]any{
		"headline": stringField(),
		"plays": arrayOf(strictObject(map[string]any{
			"name":   stringField(),
			"points": arrayOf(stringField()),
		})),
	})),
})

// focusSummary は要約の結果。Digest が nil のときは構造化に失敗しており、
// Text（LLM の生出力）をそのまま平文で投稿する。
type focusSummary struct {
	Digest *focusDigest
	Text   string
}

// ---------------------------------------------------------------- 引数解釈 ---

// newFocusJob は `focus` 以降のトークンから仕事の単位を組み立てる。
// スレッド内メンションかつ引数なしのときは、そのスレッド 1 本だけを対象にする。
func newFocusJob(args []string, now time.Time, channel, mentionTS, threadTS string) (focusJob, error) {
	job := focusJob{Channel: channel, MentionTS: mentionTS, ThreadTS: threadTS}
	if len(args) == 0 && threadTS != "" {
		job.ThreadOnly = true
		return job, nil
	}
	since, err := parseFocusSince(args, now)
	if err != nil {
		return job, err
	}
	job.Oldest = since.Unix()
	return job, nil
}

// parseFocusSince は `8/8` / `2026-08-08` / `12d` / 未指定 を解釈し、
// サービスタイムゾーン（Asia/Tokyo）の 00:00 を返す。
func parseFocusSince(args []string, now time.Time) (time.Time, error) {
	now = now.In(server.ServiceLocation)
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return startOfDay(now.AddDate(0, 0, -focusDefaultDays)), nil
	}
	arg := strings.TrimSpace(args[0])

	// Nd: N 日前から
	if m := focusDaysPattern.FindStringSubmatch(arg); m != nil {
		days, _ := strconv.Atoi(m[1])
		return startOfDay(now.AddDate(0, 0, -days)), nil
	}
	// YYYY-MM-DD
	if t, err := time.ParseInLocation("2006-01-02", arg, server.ServiceLocation); err == nil {
		return startOfDay(t), nil
	}
	// M/D: 今年として解釈し、未来日になるなら前年とみなす
	if m := focusMonthDayPatten.FindStringSubmatch(arg); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		t := time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, server.ServiceLocation)
		if t.After(now) {
			t = t.AddDate(-1, 0, 0)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("期間の指定 `%s` が読めませんでした。`8/8` `2026-08-08` `12d` のいずれかで指定してください", arg)
}

func startOfDay(t time.Time) time.Time {
	t = t.In(server.ServiceLocation)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, server.ServiceLocation)
}

// -------------------------------------------------------------- 受付と enqueue ---

// onMentionFocus は Webhook 側の入口。重い処理は一切せず、
// 「聞こえた」ことを 👀 で示してキューに積むだけで返る。
func (bot Bot) onMentionFocus(event slackevents.AppMentionEvent, args []string) {
	mention := slack.NewRefToMessage(event.Channel, event.TimeStamp)
	_ = callSlack(func() error { return bot.SlackAPI.AddReaction(focusReactionWorking, mention) })

	job, err := newFocusJob(args, time.Now(), event.Channel, event.TimeStamp, event.ThreadTimeStamp)
	if err != nil {
		bot.abortFocus(job, err)
		return
	}

	// Webhook のハンドラは既に復帰しているので req.Context() は cancel 済み。使わない。
	ctx := context.Background()

	if bot.Enqueuer == nil {
		// ローカル開発には Cloud Tasks が無いので、同プロセスでワーカーを直接呼ぶ。
		go func() { _ = bot.runFocus(ctx, job) }()
		return
	}

	payload, err := json.Marshal(job)
	if err == nil {
		err = bot.Enqueuer.Enqueue(ctx, focusTaskName(job), FocusTaskURI, payload)
	}
	if err != nil {
		bot.abortFocus(job, err)
	}
}

// focusTaskName は Cloud Tasks の task ID。同じメンションに対する二重 enqueue は
// Cloud Tasks 側で ALREADY_EXISTS として弾かれる（task ID に `.` は使えない）。
func focusTaskName(job focusJob) string {
	return "focus-" + job.Channel + "-" + strings.ReplaceAll(job.MentionTS, ".", "-")
}

// abortFocus は受付段階での失敗をスレッドに返し、👀 を外す。黙って失敗しない。
func (bot Bot) abortFocus(job focusJob, cause error) {
	bot.replyToMention(job, cause.Error())
	_ = callSlack(func() error {
		return bot.SlackAPI.RemoveReaction(focusReactionWorking, slack.NewRefToMessage(job.Channel, job.MentionTS))
	})
}

// ------------------------------------------------------------------ ワーカー ---

// FocusTask は Cloud Tasks（App Engine ターゲット）から POST される要約ワーカー。
func (bot Bot) FocusTask(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w, true)
	defer req.Body.Close()

	// stellar:debt(scope) Cloud Tasks の自動再試行を使わない（常に 200）。upgrade: payload に
	// 受付メッセージ ts を持たせ、再試行時は UpdateMessage で「再試行中」に更新して二重投稿を防ぐ
	job := focusJob{}
	if err := json.NewDecoder(req.Body).Decode(&job); err != nil {
		log.Println("[focus] decode:", err)
		render.JSON(http.StatusOK, marmoset.P{"error": err.Error()})
		return
	}
	if err := bot.runFocus(req.Context(), job); err != nil {
		log.Println("[focus]", err)
		render.JSON(http.StatusOK, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, marmoset.P{"ok": true})
}

// runFocus は要約本体を実行し、成否に応じてリアクションを差し替える。
func (bot Bot) runFocus(ctx context.Context, job focusJob) error {
	err := bot.focus(ctx, job, memberNameResolver(ctx))
	if err != nil {
		bot.replyToMention(job, err.Error())
	}
	mention := slack.NewRefToMessage(job.Channel, job.MentionTS)
	_ = callSlack(func() error { return bot.SlackAPI.RemoveReaction(focusReactionWorking, mention) })
	if err == nil {
		_ = callSlack(func() error { return bot.SlackAPI.AddReaction(focusReactionDone, mention) })
	}
	return err
}

// focus は 収集 → 進捗表示 → 要約 → 投稿 の一連。resolve は `<@Uxxxx>` の名前解決。
func (bot Bot) focus(ctx context.Context, job focusJob, resolve func(string) string) error {
	now := time.Now().In(server.ServiceLocation)

	var threads []playThread
	var statusTS string
	var err error

	if job.ThreadOnly {
		if threads, err = bot.collectSingleThread(job); err != nil {
			return err
		}
	} else {
		parents, e := bot.fetchParents(job)
		if e != nil {
			return e
		}
		if len(parents) == 0 {
			return bot.replyToMention(job, focusEmptyMessage)
		}
		plays, headlines := countParentKinds(parents)
		statusTS, err = bot.postStatus(job, fmt.Sprintf(
			"📝 %s の %d プレー（見出し %d 件）を読んでいます。1〜2 分ほどかかります",
			focusRangeLabel(job, now), plays, headlines))
		if err != nil {
			return err
		}
		if threads, err = bot.expandThreads(job, parents, bot.progressUpdater(job, statusTS)); err != nil {
			return err
		}
	}

	if len(threads) == 0 {
		return bot.replyToMention(job, focusEmptyMessage)
	}

	summary, err := bot.summarize(ctx, job, threads, resolve)
	if err != nil {
		return err
	}

	if summary.Digest != nil {
		if err := bot.postFocusMessages(job, focusMessages(job, threads, now, *summary.Digest)); err != nil {
			return err
		}
	} else {
		// 構造化に失敗したときは黙って諦めず、従来どおり平文で投稿する。
		chunks := chunkLines(focusHeader(job, threads, now)+"\n\n"+summary.Text, focusChunkSize)
		if err := bot.postSummary(job, chunks); err != nil {
			return err
		}
	}

	if statusTS != "" {
		_ = callSlack(func() error {
			_, _, _, err := bot.SlackAPI.UpdateMessage(job.Channel, statusTS,
				slack.MsgOptionText("✅ 完了", false))
			return err
		})
	}
	return nil
}

// ------------------------------------------------------------------ 収集 ---

// collectSingleThread はスレッド内メンション用。history は呼ばず、
// 指定スレッドの返信だけを取って 1 本の playThread にまとめる。
func (bot Bot) collectSingleThread(job focusJob) ([]playThread, error) {
	msgs, err := bot.fetchReplies(job.Channel, job.ThreadTS)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}
	parent := msgs[0]
	return []playThread{{
		Parent:  parent,
		Replies: filterReplies(msgs, parent.Timestamp, job.MentionTS),
	}}, nil
}

// fetchParents は conversations.history をページングしながら、
// プレー／見出しの候補となるトップレベル投稿だけを古い順で返す。
func (bot Bot) fetchParents(job focusJob) ([]slack.Message, error) {
	parents := []slack.Message{}
	cursor := ""
	for {
		var resp *slack.GetConversationHistoryResponse
		if err := callSlack(func() (err error) {
			resp, err = bot.SlackAPI.GetConversationHistory(&slack.GetConversationHistoryParameters{
				ChannelID: job.Channel,
				Oldest:    strconv.FormatInt(job.Oldest, 10),
				Limit:     focusHistoryPageLimit,
				Cursor:    cursor,
			})
			return err
		}); err != nil {
			return nil, err
		}
		for _, m := range resp.Messages {
			if isSkippableParent(m, job.MentionTS) {
				continue
			}
			parents = append(parents, m)
		}
		if cursor = resp.ResponseMetaData.NextCursor; cursor == "" {
			break
		}
	}
	// history は新しい順に返るので、投稿順（古い順）に並べ直す。
	sort.SliceStable(parents, func(i, j int) bool {
		return timestampValue(parents[i].Timestamp) < timestampValue(parents[j].Timestamp)
	})
	return parents, nil
}

// expandThreads は返信のある親について conversations.replies を引き、プレー単位に束ねる。
func (bot Bot) expandThreads(job focusJob, parents []slack.Message, progress func(done, total int)) ([]playThread, error) {
	threads := make([]playThread, 0, len(parents))
	// 進捗は「返信を取りに行った親（＝プレー）」だけで数える。見出しは replies を
	// 引かないので、これを混ぜると分子が分母（プレー数）を超えてしまう。
	plays, _ := countParentKinds(parents)
	done := 0
	for _, parent := range parents {
		thread := playThread{Parent: parent}
		if parent.ReplyCount > 0 {
			msgs, err := bot.fetchReplies(job.Channel, parent.Timestamp)
			if err != nil {
				return nil, err
			}
			thread.Replies = filterReplies(msgs, parent.Timestamp, job.MentionTS)
			done++
			if progress != nil && done%focusProgressInterval == 0 {
				progress(done, plays)
			}
		}
		threads = append(threads, thread)
	}
	return threads, nil
}

// fetchReplies は conversations.replies をページングして全返信を返す（先頭要素は親自身）。
func (bot Bot) fetchReplies(channel, threadTS string) ([]slack.Message, error) {
	msgs := []slack.Message{}
	cursor := ""
	for {
		var page []slack.Message
		var next string
		if err := callSlack(func() (err error) {
			page, _, next, err = bot.SlackAPI.GetConversationReplies(&slack.GetConversationRepliesParameters{
				ChannelID: channel,
				Timestamp: threadTS,
				Limit:     focusRepliesPageLimit,
				Cursor:    cursor,
			})
			return err
		}); err != nil {
			return nil, err
		}
		msgs = append(msgs, page...)
		if cursor = next; cursor == "" {
			break
		}
	}
	return msgs, nil
}

func filterReplies(msgs []slack.Message, parentTS, mentionTS string) []slack.Message {
	replies := []slack.Message{}
	for _, m := range msgs {
		if m.Timestamp == parentTS { // 先頭に来る親自身は返信ではない
			continue
		}
		if isBotOrSystem(m, mentionTS) {
			continue
		}
		replies = append(replies, m)
	}
	return replies
}

// isBotOrSystem は bot・システム投稿、および要約を起動したメンション自身を判定する。
// onMessage（翻訳）にも似た SubType の除外があるが、あちらは slackevents.MessageEvent が
// 対象で除外理由も異なる（翻訳の対象外にする）。統合すると翻訳の挙動が変わるため分けている。
// メンションを除外しないと、受付メッセージを返信した時点でメンションが
// 「返信のある親」になり、プレーとして要約対象に混入してしまう。
func isBotOrSystem(m slack.Message, mentionTS string) bool {
	if mentionTS != "" && m.Timestamp == mentionTS {
		return true
	}
	if m.BotID != "" {
		return true
	}
	switch m.SubType {
	case "bot_message", "channel_join", "channel_leave", "message_changed", "message_deleted":
		return true
	}
	return false
}

func isSkippableParent(m slack.Message, mentionTS string) bool {
	// thread_broadcast はスレッド返信の複製がトップレベルに出たもの。親として数えない。
	return m.SubType == "thread_broadcast" || isBotOrSystem(m, mentionTS)
}

// countParentKinds は replies を引く前に、Slack のメタ情報（ReplyCount）だけで見積もる。
// 受付メッセージを早く出すためにここでは返信本体を取らない。
func countParentKinds(parents []slack.Message) (plays, headlines int) {
	for _, p := range parents {
		if p.ReplyCount == 0 {
			headlines++
		} else {
			plays++
		}
	}
	return plays, headlines
}

// countThreadKinds は bot・システム投稿を除外したあとの実数で数える。
// bot の返信しか無い親は countParentKinds ではプレー、ここでは見出しになるため、
// 受付メッセージと最終メタ行で件数がずれることがある（実数は最終メタ行が正）。
func countThreadKinds(threads []playThread) (plays, headlines, replies int) {
	for _, t := range threads {
		if t.IsHeadline() {
			headlines++
		} else {
			plays++
		}
		replies += len(t.Replies)
	}
	return plays, headlines, replies
}

func timestampValue(ts string) float64 {
	v, _ := strconv.ParseFloat(ts, 64)
	return v
}

// callSlack は Slack の rate limit（429）だけを RetryAfter 待ちで再試行する。
func callSlack(fn func() error) error {
	var err error
	for attempt := 0; attempt < focusRateLimitRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		var limited *slack.RateLimitedError
		if !errors.As(err, &limited) {
			return err
		}
		time.Sleep(limited.RetryAfter)
	}
	return err
}

// ------------------------------------------------------------------ 要約 ---

// focusSystemPrompt は要約の指示。few は「対象が少ない」ことを表し、focus の点数を絞る。
// 出力の「形」は focusDigestSchema（Structured Outputs / strict）が縛るので、
// ここには「中身」の指示だけを書く。
func focusSystemPrompt(few bool) string {
	count := "3〜5 点"
	if few {
		count = "1〜3 点"
	}
	return `あなたはアメリカンフットボールチームのコーチ補佐です。
入力は Slack に投稿された「練習の投稿とその反省スレッド」です。
[投稿] 行が投稿本文、[投稿・返信なし] 行は返信の付いていない投稿、その下の "- 名前: 本文" が反省の書き込みです。

- focus はこのチャンネルが次の練習で意識すべき点を ` + count + ` に絞る
- 複数のプレーで繰り返し出ている指摘を優先し、count に根拠となったプレー数、plays に代表的なプレー名を最大 3 件入れる
- title は 1 行の見出し、detail は「次の練習で何を意識するか」が分かる 1〜2 行
- positions には関係するポジション（QB, WR, OL など）を入れる。特定できなければ空配列
- sections はプレー別の詳細。headline はドリルやシリーズの区切り（例: "8/29 skel"）、plays の name は投稿されたプレー名をそのまま使う
- 区切りとなる見出しが見当たらない場合は headline を空文字にした section を 1 つだけ作る
- @channel や @here を含む告知、「ナイスオフェンス！！」のような感想はプレーとして扱わず sections に含めない
- 返信の無いプレー投稿は points を空配列にして sections に残す
- 入力の並び順を保つ
- 値の文字列に Slack の装飾記号（* や _）を入れない`
}

// focusFewTargets は「対象が少ない」入力かどうかを返す。スレッド単体要約、または
// 返信の付いた投稿が focusFewTargetsThreshold 件未満のとき true。
func focusFewTargets(job focusJob, threads []playThread) bool {
	if job.ThreadOnly {
		return true
	}
	plays, _, _ := countThreadKinds(threads)
	return plays < focusFewTargetsThreshold
}

// summarize は全スレッドを 1 プロンプトにまとめて 1 回だけ ChatGPT を呼ぶ。
// 返答は Structured Outputs（focusDigestSchema）で JSON に縛るが、それでも
// parse できないときは失敗させず、生のテキストを平文フォールバックとして返す。
func (bot Bot) summarize(ctx context.Context, job focusJob, threads []playThread, resolve func(string) string) (focusSummary, error) {
	groups := splitThreadsForPrompt(threads, focusPromptRuneBudget)
	prompt := focusSystemPrompt(focusFewTargets(job, threads))
	parts := make([]string, 0, len(groups))
	digest := focusDigest{}
	structured := true // 1 塊でも parse に失敗したら平文フォールバックに倒す
	for _, group := range groups {
		reply, err := bot.ChatGPT.Chat(ctx, ChatRequest{
			Model:  chatModelFocus,
			System: []string{prompt},
			User:   renderThreads(group, resolve),
			Schema: &ChatJSONSchema{Name: focusDigestSchemaName, Schema: focusDigestSchema},
		})
		if err != nil {
			return focusSummary{}, err
		}
		content := strings.TrimSpace(reply)
		parts = append(parts, content)
		if !structured {
			continue // 平文フォールバックが確定済み。残りは Text を組むためだけに読む
		}
		part := focusDigest{}
		if err := json.Unmarshal([]byte(content), &part); err != nil {
			log.Printf("[focus] structured output を受け取れませんでした, falling back to plain text: %v", err)
			structured = false
			continue
		}
		digest.Focus = append(digest.Focus, part.Focus...)
		digest.Sections = append(digest.Sections, part.Sections...)
	}
	summary := focusSummary{Text: strings.Join(parts, "\n\n")}
	if !structured {
		return summary, nil
	}
	if !digest.valid() {
		log.Printf("[focus] digest has no focus points, falling back to plain text")
		return summary, nil
	}
	summary.Digest = &digest
	return summary, nil
}

// splitThreadsForPrompt は入力が文脈長に収まる限り 1 塊のまま返す。
// stellar:debt(scope) 分割時は focus が塊ごとに出て 3〜5 点に収束しない。upgrade: 2 段目 reduce
func splitThreadsForPrompt(threads []playThread, budget int) [][]playThread {
	sizes := make([]int, len(threads))
	total := 0
	for i, t := range threads {
		sizes[i] = threadRuneCount(t)
		total += sizes[i]
	}
	if total <= budget {
		return [][]playThread{threads}
	}
	groups := [][]playThread{}
	current := []playThread{}
	size := 0
	for i, t := range threads {
		n := sizes[i]
		if len(current) > 0 && size+n > budget {
			groups = append(groups, current)
			current, size = nil, 0
		}
		current = append(current, t)
		size += n
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func threadRuneCount(t playThread) int {
	n := utf8.RuneCountInString(t.Parent.Text)
	for _, r := range t.Replies {
		n += utf8.RuneCountInString(r.Text)
	}
	return n
}

// renderThreads はプロンプトに渡す平文へ整形する。`<@Uxxxx>` は表示名に置換する。
// プレーか見出しかを Hub 側で断定せず、「返信が無い」という事実だけを伝えて判定は
// LLM に委ねる（返信の有無だけで告知がプレーに、返信 0 のプレーが見出しに化けるため）。
func renderThreads(threads []playThread, resolve func(string) string) string {
	buf := &strings.Builder{}
	for _, t := range threads {
		text := resolveMentions(strings.TrimSpace(t.Parent.Text), resolve)
		if len(t.Replies) == 0 {
			fmt.Fprintf(buf, "\n[投稿・返信なし] %s\n", text)
			continue
		}
		fmt.Fprintf(buf, "\n[投稿] %s\n", text)
		for _, r := range t.Replies {
			body := strings.TrimSpace(resolveMentions(r.Text, resolve))
			if body == "" {
				continue
			}
			fmt.Fprintf(buf, "  - %s: %s\n", resolveUser(r.User, resolve), strings.ReplaceAll(body, "\n", " "))
		}
	}
	return buf.String()
}

func resolveMentions(s string, resolve func(string) string) string {
	if resolve == nil {
		return s
	}
	return focusMentionPattern.ReplaceAllStringFunc(s, func(match string) string {
		id := match[2 : len(match)-1] // `<@Uxxxx>` の中身。パターン上この形しか来ない
		if name := resolve(id); name != "" && name != id {
			return "@" + name
		}
		return match
	})
}

func resolveUser(id string, resolve func(string) string) string {
	if id == "" {
		return "不明"
	}
	if resolve == nil {
		return id
	}
	if name := resolve(id); name != "" {
		return name
	}
	return id
}

// memberNameResolver は Datastore のメンバーキャッシュを引く本番用 resolver。
// 解決できないときは ID をそのまま返す（要約は続行する）。
func memberNameResolver(ctx context.Context) func(string) string {
	return func(id string) string {
		member, err := models.GetMemberInfoByCache(ctx, id)
		if err != nil {
			return id
		}
		if name := member.Name(); name != "" {
			return name
		}
		return id
	}
}

// ------------------------------------------------------------------ 投稿 ---

// chunkLines は Slack の 1 メッセージ上限に収まるよう、行頭でのみ分割する。
func chunkLines(s string, max int) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	segments := []string{}
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		segments = append(segments, splitLongLine(line, max)...)
	}
	chunks := []string{}
	buf := []string{}
	size := 0
	for _, seg := range segments {
		n := utf8.RuneCountInString(seg)
		if len(buf) > 0 && size+1+n > max {
			chunks = append(chunks, strings.Join(buf, "\n"))
			buf, size = nil, 0
		}
		if len(buf) == 0 {
			size = n
		} else {
			size += 1 + n
		}
		buf = append(buf, seg)
	}
	if len(buf) > 0 {
		chunks = append(chunks, strings.Join(buf, "\n"))
	}
	return chunks
}

// splitLongLine は 1 行が上限を超える場合だけ、rune 境界で強制分割する。
func splitLongLine(line string, max int) []string {
	if utf8.RuneCountInString(line) <= max {
		return []string{line} // ほとんどの行はここで返る（[]rune 変換を避ける）
	}
	runes := []rune(line)
	out := []string{}
	for len(runes) > max {
		out = append(out, string(runes[:max]))
		runes = runes[max:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

// postSummary は平文フォールバック用。チャンクを 1 通ずつの focusMessage に均して
// postFocusMessages に流す（連投・broadcast の規則を 1 箇所に閉じる）。
func (bot Bot) postSummary(job focusJob, chunks []string) error {
	msgs := make([]focusMessage, 0, len(chunks))
	for _, chunk := range chunks {
		msgs = append(msgs, focusMessage{Text: chunk})
	}
	return bot.postFocusMessages(job, msgs)
}

// postFocusMessages は要約をメンションのスレッドへ連投する。
// トップレベルにも見せたいのは 1 通目（focus の digest）だけなので broadcast はそこにしか
// 付けない。blocks だけの投稿は通知・検索が空になるので Text を必ず併せて渡す。
func (bot Bot) postFocusMessages(job focusJob, msgs []focusMessage) error {
	for i, msg := range msgs {
		opts := []slack.MsgOption{
			slack.MsgOptionText(msg.Text, false),
			slack.MsgOptionTS(job.MentionTS),
		}
		if len(msg.Blocks) > 0 {
			// 空スライスで呼ぶと blocks=[] を送って既存 blocks を消す挙動になるので渡さない。
			opts = append(opts, slack.MsgOptionBlocks(msg.Blocks...))
		}
		if i == 0 && !job.ThreadOnly {
			opts = append(opts, slack.MsgOptionBroadcast())
		}
		if err := callSlack(func() error {
			_, _, err := bot.SlackAPI.PostMessage(job.Channel, opts...)
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// postStatus は範囲確定メッセージをスレッドへ投稿し、以降 UpdateMessage で書き換える ts を返す。
func (bot Bot) postStatus(job focusJob, text string) (string, error) {
	var ts string
	err := callSlack(func() error {
		_, t, err := bot.SlackAPI.PostMessage(job.Channel,
			slack.MsgOptionText(text, false), slack.MsgOptionTS(job.MentionTS))
		ts = t
		return err
	})
	return ts, err
}

func (bot Bot) progressUpdater(job focusJob, statusTS string) func(done, total int) {
	if statusTS == "" {
		return nil
	}
	return func(done, total int) {
		_ = callSlack(func() error {
			_, _, _, err := bot.SlackAPI.UpdateMessage(job.Channel, statusTS, slack.MsgOptionText(
				fmt.Sprintf("📝 %d プレー中 %d 件のスレッドを読みました…", total, done), false))
			return err
		})
	}
}

func (bot Bot) replyToMention(job focusJob, text string) error {
	return callSlack(func() error {
		_, _, err := bot.SlackAPI.PostMessage(job.Channel,
			slack.MsgOptionText(text, false), slack.MsgOptionTS(job.MentionTS))
		return err
	})
}

func focusRangeLabel(job focusJob, now time.Time) string {
	if job.ThreadOnly {
		return "このスレッド"
	}
	from := time.Unix(job.Oldest, 0).In(server.ServiceLocation)
	now = now.In(server.ServiceLocation)
	return fmt.Sprintf("%d/%d〜%d/%d", int(from.Month()), from.Day(), int(now.Month()), now.Day())
}

func focusHeader(job focusJob, threads []playThread, now time.Time) string {
	plays, _, replies := countThreadKinds(threads)
	if job.ThreadOnly {
		return fmt.Sprintf("*このスレッドの %d 件の返信を要約*", replies)
	}
	return fmt.Sprintf("*%s の %d プレー / %d 件の返信を要約*", focusRangeLabel(job, now), plays, replies)
}
