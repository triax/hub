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
	"github.com/otiai10/openaigo"
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

	focusReactionWorking = "eyes"
	focusReactionDone    = "white_check_mark"

	focusEmptyMessage = "対象の投稿がありませんでした"
)

var focusMentionPattern = regexp.MustCompile(`<@([A-Za-z0-9]+)>`)

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
func (t playThread) IsHeadline() bool { return len(t.Replies) == 0 }

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
	if m := regexp.MustCompile(`^(\d+)d$`).FindStringSubmatch(arg); m != nil {
		days, _ := strconv.Atoi(m[1])
		return startOfDay(now.AddDate(0, 0, -days)), nil
	}
	// YYYY-MM-DD
	if t, err := time.ParseInLocation("2006-01-02", arg, server.ServiceLocation); err == nil {
		return startOfDay(t), nil
	}
	// M/D: 今年として解釈し、未来日になるなら前年とみなす
	if m := regexp.MustCompile(`^(\d{1,2})/(\d{1,2})$`).FindStringSubmatch(arg); m != nil {
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
		if threads, err = bot.collectThreads(job, nil); err != nil {
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
		if threads, err = bot.expandThreads(job, parents, bot.progressUpdater(job, statusTS, plays)); err != nil {
			return err
		}
	}

	if len(threads) == 0 {
		return bot.replyToMention(job, focusEmptyMessage)
	}

	summary, err := bot.summarize(ctx, threads, resolve)
	if err != nil {
		return err
	}

	chunks := chunkLines(focusHeader(job, threads, now)+"\n\n"+summary, focusChunkSize)
	if err := bot.postSummary(job, chunks); err != nil {
		return err
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

// collectThreads は対象期間の親投稿と、その全返信を古い順に集める。
func (bot Bot) collectThreads(job focusJob, progress func(done, total int)) ([]playThread, error) {
	if job.ThreadOnly {
		// history は呼ばず、指定スレッドの返信だけを取る。
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
	parents, err := bot.fetchParents(job)
	if err != nil {
		return nil, err
	}
	return bot.expandThreads(job, parents, progress)
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
	for i, parent := range parents {
		thread := playThread{Parent: parent}
		if parent.ReplyCount > 0 {
			msgs, err := bot.fetchReplies(job.Channel, parent.Timestamp)
			if err != nil {
				return nil, err
			}
			thread.Replies = filterReplies(msgs, parent.Timestamp, job.MentionTS)
		}
		threads = append(threads, thread)
		if progress != nil && (i+1)%focusProgressInterval == 0 {
			progress(i+1, len(parents))
		}
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

const focusSystemPrompt = `あなたはアメリカンフットボールチームのコーチ補佐です。
入力は Slack に投稿された「プレーごとの反省スレッド」です。
[プレー] 行がプレー名、その下の "- 名前: 本文" が反省の書き込み、[見出し] 行はドリルやシリーズの区切りです。

次のルールで要約してください:
- プレーごとに 1〜3 行で、次の練習で意識することが分かるように書く
- [見出し] 行は区切りとしてそのまま残す
- プレー名は書き換えず、投稿されたとおりに記載する
- 入力の並び順を保つ
- 最後に「共通課題」として、全体を通して繰り返し出ている問題を最大 3 点あげる
- Slack に投稿するので mrkdwn（*太字* など）で読みやすく整える`

// summarize は全スレッドを 1 プロンプトにまとめて 1 回だけ ChatGPT を呼ぶ。
func (bot Bot) summarize(ctx context.Context, threads []playThread, resolve func(string) string) (string, error) {
	groups := splitThreadsForPrompt(threads, focusPromptRuneBudget)
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		res, err := bot.ChatGPT.Chat(ctx, openaigo.ChatRequest{
			Model: openaigo.GPT4o,
			Messages: []openaigo.Message{
				{Role: "system", Content: focusSystemPrompt},
				{Role: "user", Content: renderThreads(group, resolve)},
			},
		})
		if err != nil {
			return "", err
		}
		if len(res.Choices) == 0 {
			return "", fmt.Errorf("要約が返ってきませんでした")
		}
		parts = append(parts, strings.TrimSpace(res.Choices[0].Message.Content))
	}
	return strings.Join(parts, "\n\n"), nil
}

// splitThreadsForPrompt は入力が文脈長に収まる限り 1 塊のまま返す。
// stellar:debt(scope) 分割時は共通課題が塊ごとに出る。upgrade: 2 段目 reduce
func splitThreadsForPrompt(threads []playThread, budget int) [][]playThread {
	total := 0
	for _, t := range threads {
		total += threadRuneCount(t)
	}
	if total <= budget {
		return [][]playThread{threads}
	}
	groups := [][]playThread{}
	current := []playThread{}
	size := 0
	for _, t := range threads {
		n := threadRuneCount(t)
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
func renderThreads(threads []playThread, resolve func(string) string) string {
	buf := &strings.Builder{}
	for _, t := range threads {
		text := resolveMentions(strings.TrimSpace(t.Parent.Text), resolve)
		if t.IsHeadline() {
			fmt.Fprintf(buf, "\n[見出し] %s\n", text)
			continue
		}
		fmt.Fprintf(buf, "\n[プレー] %s\n", text)
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
		id := focusMentionPattern.FindStringSubmatch(match)[1]
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
	runes := []rune(line)
	if len(runes) <= max {
		return []string{line}
	}
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

// postSummary はメンションのスレッドへ要約を連投する。
// トップレベルにも見せたいのは 1 チャンク目だけなので、broadcast はそこにしか付けない。
func (bot Bot) postSummary(job focusJob, chunks []string) error {
	for i, chunk := range chunks {
		opts := []slack.MsgOption{
			slack.MsgOptionText(chunk, false),
			slack.MsgOptionTS(job.MentionTS),
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

func (bot Bot) progressUpdater(job focusJob, statusTS string, plays int) func(done, total int) {
	if statusTS == "" {
		return nil
	}
	return func(done, total int) {
		_ = callSlack(func() error {
			_, _, _, err := bot.SlackAPI.UpdateMessage(job.Channel, statusTS, slack.MsgOptionText(
				fmt.Sprintf("📝 %d プレー中 %d 件のスレッドを読みました…", plays, done), false))
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
