package slackbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/otiai10/largo"
	"github.com/otiai10/marmoset"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/triax/hub/server"
	"github.com/triax/hub/server/models"
)

// premortem は focus の裏返し。focus が「すでに起きたこと」を要約するのに対し、
// premortem は「次の試合に負けたと仮定して、その死因を先に挙げる」（#683）。
// 収集・投稿・進捗の段取りは focus と同じなので、そこは focus 側の実装を呼ぶ。

const (
	// PremortemTaskURI は Cloud Tasks から叩かれるワーカーの相対パス。
	// キューは focus と同じものを使う（CLOUD_TASKS_QUEUE / 既定 DefaultCloudTasksQueue）。
	PremortemTaskURI = "/tasks/premortem"

	// 収集チャンネルの上限。指定が増えるほど 1 プロンプトが膨らんで分割されやすくなるので、
	// 際限なく受けない。
	premortemMaxSourceChannels = 5

	premortemEmptyMessage = "対象の投稿がありませんでした"
)

// premortemKind は「死因の型」。出力には一切出さず、候補の網羅性と採用枠の配り方にだけ使う。
const (
	premortemKindExecution   = "execution"   // 練習でできていることが試合で出ない
	premortemKindPreparation = "preparation" // 対策そのものが用意されていなかった
	premortemKindHypothesis  = "hypothesis"  // 相手・展開の事前の想定が外れた
	premortemKindMatchup     = "matchup"     // 単純な個の力負け
	premortemKindAdjust      = "adjust"      // 現場で把握できず、正しいカードを切り替えられなかった
)

var (
	// premortemKinds は schema の enum。並びがそのままプロンプトの説明順になる。
	premortemKinds = []string{
		premortemKindExecution, premortemKindPreparation,
		premortemKindHypothesis, premortemKindMatchup, premortemKindAdjust,
	}
	// premortemJudgementKinds は「判断」の死因。準備と実力の話（他の 3 つ）と違って
	// 明示的に要求しないと出てこないので、出たかどうかを記録する対象にする。
	premortemJudgementKinds = []string{premortemKindHypothesis, premortemKindAdjust}

	// Slack はメンション本文で `#scouting` を `<#C0ABCDEF|scouting>` に展開する。
	premortemChannelPattern = regexp.MustCompile(`^<#([A-Za-z0-9]+)(?:\|[^>]*)?>$`)
	// タイトル先頭の `#試合` タグは見出しに出すとノイズなので落とす。
	premortemGameTagPattern = regexp.MustCompile(`[＃#]試合`)
)

// premortemJob は Webhook（enqueue 側）とワーカー（/tasks/premortem）の間で受け渡す仕事の単位。
type premortemJob struct {
	// Channel はメンションされたチャンネル。投稿先であり、収集元の 1 つでもある。
	Channel string `json:"channel"`
	// Sources は収集対象チャンネル（先頭は必ず Channel）。`#scouting` の指定で増える。
	Sources []string `json:"sources"`
	// MentionTS は起動したメッセージの ts。mention のときだけ埋まる（slash には
	// アンカーになるメッセージが無い。#695）。リアクション・スレッド・除外に使う。
	MentionTS  string `json:"mention_ts"`
	ThreadTS   string `json:"thread_ts"`
	Oldest     int64  `json:"oldest"`
	ThreadOnly bool   `json:"thread_only"`
	// Ephemeral は「打った人だけに見える」配送。slash（/premortem）で true になる。
	Ephemeral bool `json:"ephemeral"`
	// UserID は Ephemeral のときの宛先。
	UserID string `json:"user_id"`
	// TaskKey は Cloud Tasks の task ID を一意にする種。mention は MentionTS、
	// slash は trigger_id（1 起動ごとに一意）。
	TaskKey string `json:"task_key"`
}

// focusJobFor は収集・投稿系のヘルパ（fetchParents / expandThreads / postStatus 等）へ
// 渡すためのアダプタ。#683 決定 2 は「まずコピーして独立実装」だが、Slack のページングと
// レート制限の扱いまで二重化すると #664 / #668 のような修正が片方に取り残される。
// job 変換 1 枚だけを挟んで収集層を再利用し、共通化そのものは別 Issue に切る。
func (job premortemJob) focusJobFor(channel string) focusJob {
	return focusJob{
		Channel:    channel,
		MentionTS:  job.MentionTS,
		ThreadTS:   job.ThreadTS,
		Oldest:     job.Oldest,
		ThreadOnly: job.ThreadOnly,
	}
}

// premortemRisk は死因の候補 1 件。Key は plays から参照するための識別子。
type premortemRisk struct {
	Key string `json:"key"`
	// Kind は死因の型（premortemKinds の enum）。採用枠の配り方にだけ使い、描画しない（#683）。
	Kind  string `json:"kind"`
	Title string `json:"title"`
	// Label は目次と pie の凡例に出す短い名詞句（#681 と同じ理由で Title とは別に持つ）。
	Label string `json:"label"`
	// Opponent は相手側の前提を述べた 1 文。材料が無ければ空。**相手の情報は上乗せであって
	// premortem の土台ではない**（#698 大原則）ので、空でも出力は成立しなければならない。
	Opponent string `json:"opponent"`
	// OpponentQuote は Opponent の根拠になった入力の原文。プロンプトに渡した平文に逐語で
	// 現れなければ Opponent ごと落とす（#698 決定 3・決定 10）。
	OpponentQuote string `json:"opponent_quote"`
	// Scenario はどう崩れるかの 1〜2 文。Opponent に続けて読んで意味が通る形で書かせるが、
	// Opponent が落ちても単体で成立する。
	Scenario string `json:"scenario"`
	// Phase は局面（3rd&long / レッドゾーン等）。チャンネルからは決まらないので常に描く。
	Phase string `json:"phase"`
	// Unit は OF / DF / ST。#offence 等スコープの定まったチャンネルで打たれるのが基本なので、
	// 採用リスクが 2 ユニット以上にまたがるときだけ描く。
	Unit string `json:"unit"`
	// Signal は「試合中に何が見えたら赤信号か」。1 通目の末尾に集約して描く。
	Signal string `json:"signal"`
	// Prevent は今週やること（1〜2 件）。件数は schema では縛れないので normalize で切る。
	Prevent   []string `json:"prevent"`
	Positions []string `json:"positions"`
	Quote     string   `json:"quote"`
}

// premortemPlay は根拠になったプレー。順位付け（risk_keys の実数）にだけ使う。
// focus の focusPlay と違い headline / issue / positions は持たない。premortem には
// `full` 相当のプレー別詳細もチャートも無く、持たせても誰も読まないため（#683 決定 G-3）。
type premortemPlay struct {
	Name     string   `json:"name"`
	RiskKeys []string `json:"risk_keys"`
}

// premortemDigest は LLM に返させる構造化出力。件数は Hub 側で数えるので持たせない。
type premortemDigest struct {
	Risks []premortemRisk `json:"risks"`
	Plays []premortemPlay `json:"plays"`
}

const premortemReportSchemaName = "premortem_report"

// enumField は取りうる値を固定する string field。strict schema でも enum は使える。
func enumField(values ...string) map[string]any {
	items := make([]any, 0, len(values))
	for _, v := range values {
		items = append(items, v)
	}
	return map[string]any{"type": "string", "enum": items}
}

var premortemReportSchema = strictObject(map[string]any{
	"risks": arrayOf(strictObject(map[string]any{
		"key":            stringField(),
		"kind":           enumField(premortemKinds...),
		"title":          stringField(),
		"label":          stringField(),
		"opponent":       stringField(),
		"opponent_quote": stringField(),
		"scenario":       stringField(),
		"phase":          stringField(),
		"unit":           stringField(),
		"signal":         stringField(),
		// prevent の件数は strict schema（minItems / maxItems 非対応）では縛れない。
		"prevent":   arrayOf(stringField()),
		"positions": arrayOf(stringField()),
		"quote":     stringField(),
	})),
	"plays": arrayOf(strictObject(map[string]any{
		"name":      stringField(),
		"risk_keys": arrayOf(stringField()),
	})),
})

// premortemSummary は要約の結果。Report が nil のときは構造化に失敗しており、
// Text（LLM の生出力）をそのまま平文で投稿する。
type premortemSummary struct {
	Report *premortemReport
	Text   string
}

// ---------------------------------------------------------------- 引数解釈 ---

// newPremortemJob は `premortem` 以降のトークンから仕事の単位を組み立てる。
// 期間の解釈は focus と同じ規則。`<#C…|name>` 形式のトークンは収集対象チャンネルとして拾う。
func newPremortemJob(args []string, now time.Time, channel, mentionTS, threadTS string) (premortemJob, error) {
	args, sources := takePremortemChannels(args)
	job := premortemJob{
		Channel:   channel,
		Sources:   append([]string{channel}, sources...),
		MentionTS: mentionTS,
		ThreadTS:  threadTS,
	}
	job.Sources = uniqueStrings(job.Sources)
	if len(job.Sources) > premortemMaxSourceChannels {
		return job, fmt.Errorf("収集チャンネルは %d 件までにしてください（%d 件指定されています）",
			premortemMaxSourceChannels, len(job.Sources))
	}
	// スレッド内メンションで期間の指定が無ければ、そのスレッド 1 本だけを対象にする（focus と同じ）。
	// ただし他チャンネルが指定されているときは「このスレッドだけ」と両立しないので、
	// 期間指定として扱う（指定を黙って無視しない）。
	if len(args) == 0 && threadTS != "" && len(job.Sources) == 1 {
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

// takePremortemChannels は `<#C…>` 形式のトークンをチャンネル ID として抜き取る。位置は問わない。
func takePremortemChannels(args []string) ([]string, []string) {
	kept := make([]string, 0, len(args))
	channels := []string{}
	for _, arg := range args {
		if m := premortemChannelPattern.FindStringSubmatch(strings.TrimSpace(arg)); m != nil {
			channels = append(channels, m[1])
			continue
		}
		kept = append(kept, arg)
	}
	return kept, channels
}

// -------------------------------------------------------------- 受付と enqueue ---

func (bot Bot) onMentionPremortem(event slackevents.AppMentionEvent, args []string) {
	bot.startPremortem(args, event.Channel, event.TimeStamp, event.ThreadTimeStamp)
}

// onSlashPremortem は `/premortem` の入口。**結果は打った人だけに見える**（#695）。
//
// チャンネルには何も残さない。#694 ではアンカー投稿を出してその ts を MentionTS に使ったが、
// ephemeral 配送ではスレッドもリアクションも成立しないので、アンカーごと廃止した。
//
// 受付の ephemeral が「このチャンネルで喋れるか」の疎通確認を兼ねる。失敗＝bot 未参加なので、
// enqueue せずに response_url でユーザへ返す（チャンネルに投稿できない以上、返せる先はそこだけ）。
func (bot Bot) onSlashPremortem(w http.ResponseWriter, req *http.Request) {
	// MentionTS は空。slash にはアンカーになるメッセージが無い（それが ephemeral 配送の前提）。
	job, err := newPremortemJob(largo.Tokenize(req.Form.Get("text")), time.Now(),
		req.Form.Get("channel_id"), "", "")
	if err != nil {
		postSlackJSON(req.Form.Get("response_url"), err.Error())
		w.WriteHeader(http.StatusOK)
		return
	}
	job.Ephemeral = true
	job.UserID = req.Form.Get("user_id")
	job.TaskKey = req.Form.Get("trigger_id") // 1 起動ごとに一意。二重 enqueue の抑止に使う

	sink := bot.premortemSinkFor(job)
	if err := sink.Receipt(premortemEphemeralReceipt(len(job.Sources))); err != nil {
		log.Println("[premortem] slash receipt:", err)
		postSlackJSON(req.Form.Get("response_url"),
			"このチャンネルで実行できませんでした。bot が参加しているか確認してください。")
		w.WriteHeader(http.StatusOK)
		return
	}

	// 重い処理は enqueue の先。ここでやるのは ephemeral 1 通 + enqueue だけなので 3 秒に収まる。
	bot.enqueuePremortem(job)
	w.WriteHeader(http.StatusOK)
}

// startPremortem は mention の受付。mentionTS は起動したメンション自身の ts。
func (bot Bot) startPremortem(args []string, channel, mentionTS, threadTS string) {
	job, err := newPremortemJob(args, time.Now(), channel, mentionTS, threadTS)
	job.TaskKey = mentionTS

	sink := bot.premortemSinkFor(job)
	sink.Working()
	if err != nil {
		sink.Notice(err.Error())
		sink.Close(false)
		return
	}
	bot.enqueuePremortem(job)
}

// enqueuePremortem は仕事をキューへ積む。Cloud Tasks が無い環境（ローカル開発）では
// 同プロセスでワーカーを直接呼ぶ。
func (bot Bot) enqueuePremortem(job premortemJob) {
	// Webhook / slash のハンドラは既に復帰しているので req.Context() は cancel 済み。使わない。
	ctx := context.Background()

	if bot.Enqueuer == nil {
		bot.spawn("runPremortem", func() { _ = bot.runPremortem(ctx, job) })
		return
	}

	payload, err := json.Marshal(job)
	if err == nil {
		err = bot.Enqueuer.Enqueue(ctx, premortemTaskName(job), PremortemTaskURI, payload)
	}
	if err != nil {
		sink := bot.premortemSinkFor(job)
		sink.Notice(err.Error())
		sink.Close(false)
	}
}

// premortemTaskName は Cloud Tasks の task ID。focus と同じキューを共用するので、
// prefix で名前空間を分ける（task ID に `.` は使えない）。
func premortemTaskName(job premortemJob) string {
	return "premortem-" + job.Channel + "-" + strings.ReplaceAll(job.TaskKey, ".", "-")
}

// ------------------------------------------------------------------ ワーカー ---

// PremortemTask は Cloud Tasks（App Engine ターゲット）から POST されるワーカー。
func (bot Bot) PremortemTask(w http.ResponseWriter, req *http.Request) {
	render := marmoset.Render(w, true)
	defer req.Body.Close()

	// stellar:debt(scope) focus と同じく Cloud Tasks の自動再試行を使わない（常に 200）。
	// upgrade: payload に受付メッセージ ts を持たせ、再試行時は UpdateMessage で二重投稿を防ぐ
	job := premortemJob{}
	if err := json.NewDecoder(req.Body).Decode(&job); err != nil {
		log.Println("[premortem] decode:", err)
		render.JSON(http.StatusOK, marmoset.P{"error": err.Error()})
		return
	}
	if err := bot.runPremortem(req.Context(), job); err != nil {
		log.Println("[premortem]", err)
		render.JSON(http.StatusOK, marmoset.P{"error": err.Error()})
		return
	}
	render.JSON(http.StatusOK, marmoset.P{"ok": true})
}

func (bot Bot) runPremortem(ctx context.Context, job premortemJob) error {
	sink := bot.premortemSinkFor(job)
	err := bot.premortem(ctx, job, sink, memberNameResolver(ctx))
	if err != nil {
		sink.Notice(err.Error())
	}
	sink.Close(err == nil)
	return err
}

// premortem は 収集 → 進捗表示 → 要約 → 投稿 の一連。
func (bot Bot) premortem(ctx context.Context, job premortemJob, sink premortemSink, resolve func(string) string) error {
	now := time.Now().In(server.ServiceLocation)

	threads, unreadable, err := bot.collectPremortem(job, sink, now)
	if err != nil {
		return err
	}
	if len(unreadable) > 0 {
		// 黙って握り潰すと「読んだつもり」の premortem が出る。読めなかった事実を必ず残す。
		sink.Notice(fmt.Sprintf(
			"⚠️ %s は読めませんでした（bot が参加していない可能性があります）。残りのチャンネルだけで進めます",
			strings.Join(unreadable, ", ")))
	}
	if len(threads) == 0 {
		sink.Notice(premortemEmptyMessage)
		return nil
	}

	// 相手とチャンネルのスコープは**要約より先に**引く。ここが後段だと LLM は相手が誰かを
	// 知らないまま負け筋を挙げることになり、どの対戦相手でも同じ出力になる（#698）。
	pc := bot.premortemPromptContextFor(ctx, job, now)

	summary, err := bot.summarizePremortem(ctx, job, sink, threads, resolve, pc)
	if err != nil {
		return err
	}

	game := pc.Game
	msgs := []premortemMessage{}
	if summary.Report != nil {
		msgs = premortemMessages(job, threads, game, now, *summary.Report)
	} else {
		// 構造化に失敗したときは黙って諦めず、LLM の生出力をそのまま平文で流す。
		for _, chunk := range chunkLines(
			premortemHeader(job, threads, game, now)+"\n\n"+summary.Text, focusChunkSize) {
			msgs = append(msgs, premortemMessage{Text: chunk})
		}
	}
	if err := sink.Deliver(msgs); err != nil {
		return err
	}

	sink.Finish(premortemDoneText(job, threads, now, time.Since(now)))
	return nil
}

// ------------------------------------------------------------------ 収集 ---

// collectPremortem は Sources の各チャンネルから投稿と反省スレッドを集める。
// 1 チャンネルが読めなくても他は続行し、読めなかったチャンネルを返す（#683）。
// 全チャンネルが読めなかったときだけエラーにする。
func (bot Bot) collectPremortem(job premortemJob, sink premortemSink, now time.Time) (threads []playThread, unreadable []string, err error) {
	base := job.focusJobFor(job.Channel)

	if job.ThreadOnly {
		threads, err = bot.collectSingleThread(base)
		if err != nil {
			return nil, nil, err
		}
		if len(threads) > 0 {
			_, _, replies := countThreadKinds(threads)
			if err := sink.Receipt(fmt.Sprintf(
				"🧨 このスレッドの %d 件の返信から、負け筋を洗い出しています", replies)); err != nil {
				return nil, nil, err
			}
		}
		return threads, nil, nil
	}

	// 受付メッセージは収集前に出す（複数チャンネルだと収集そのものに時間がかかるため）。
	// ただし slash（Ephemeral）は**ハンドラが既に受付 ephemeral を出している**。あれは
	// 「このチャンネルで喋れるか」の疎通確認を兼ねていて、失敗したら enqueue しない。
	// ここで出すと同じことを 2 回言うことになるので出さない（#695）。
	if !job.Ephemeral {
		if err := sink.Receipt(fmt.Sprintf(
			"🧨 %s の %d チャンネルを読んでいます。1〜2 分ほどかかります",
			focusRangeLabel(base, now), len(job.Sources))); err != nil {
			return nil, nil, err
		}
	}

	var lastErr error
	for _, channel := range job.Sources {
		sub := job.focusJobFor(channel)
		parents, e := bot.fetchParents(sub)
		if e != nil {
			// not_in_channel 等。読めなかったチャンネルを記録して次へ。
			lastErr = e
			unreadable = append(unreadable, channelMention(channel))
			continue
		}
		expanded, e := bot.expandThreads(sub, parents, sink.Progress)
		if e != nil {
			lastErr = e
			unreadable = append(unreadable, channelMention(channel))
			continue
		}
		threads = append(threads, expanded...)
	}
	if len(unreadable) == len(job.Sources) && lastErr != nil {
		return nil, unreadable, lastErr
	}
	return threads, unreadable, nil
}

// ------------------------------------------------------------------ 対象試合 ---

// premortemPromptContext は要約プロンプトに渡す「入力の外側」の文脈。次の試合が誰で、
// いま読んでいるのがどのスコープのチャンネルか。#683 では対象試合を見出し用の表示名としか
// 扱っておらず、要約の後に引いていたため、LLM は相手が誰かもチャンネルがどこかも
// 知らないまま負け筋を挙げていた（#698）。
//
// どちらも引けないことがある（試合が無い / bot がチャンネル情報を読めない）ので、
// 空でもプロンプトが成立する形にしておく。
type premortemPromptContext struct {
	// Game は対象試合の表示名。相手名の切り出しはしない（#698 決定 9）。
	Game string
	// Scope は打たれたチャンネルの名前。ここがスコープを規定する。
	Scope string
	// Extras は追加 source チャンネルの名前。参考として読んでいるもの。
	Extras []string
}

// premortemPromptContextFor は対象試合とチャンネル名を集める。どちらの失敗も premortem を
// 止めない（相手軸が薄くなるだけで、premortem 自体は成立する = #698 大原則）。
func (bot Bot) premortemPromptContextFor(ctx context.Context, job premortemJob, now time.Time) premortemPromptContext {
	pc := premortemPromptContext{
		Game:  bot.upcomingGame(ctx, now),
		Scope: bot.channelName(job.Channel),
	}
	for _, id := range job.Sources {
		if id == job.Channel {
			continue
		}
		if name := bot.channelName(id); name != "" {
			pc.Extras = append(pc.Extras, "#"+name)
		}
	}
	return pc
}

// channelName はチャンネル ID から名前を引く。読めなければ空文字を返す（描画側の
// `<#C…>` と違い、プロンプトには ID を見せても意味が無いので落とす）。
func (bot Bot) channelName(id string) string {
	ch, err := bot.SlackAPI.GetConversationInfo(&slack.GetConversationInfoInput{ChannelID: id})
	if err != nil {
		log.Println("[premortem] GetConversationInfo:", err)
		return ""
	}
	return ch.Name
}

// upcomingGame は見出しに出す「次の試合」の表示名。引けなければ空文字を返し、
// premortem は期間ラベルにフォールバックして最後まで走る（#683）。
func (bot Bot) upcomingGame(ctx context.Context, now time.Time) string {
	event, ok, err := models.FindUpcomingGame(ctx, now, models.UpcomingGameLookahead)
	if err != nil {
		log.Println("[premortem] FindUpcomingGame:", err)
		return ""
	}
	if !ok {
		return ""
	}
	start := event.Google.Start().In(server.ServiceLocation)
	title := strings.TrimSpace(premortemGameTagPattern.ReplaceAllString(event.Google.Title, ""))
	label := fmt.Sprintf("%d/%d(%s)", int(start.Month()), start.Day(), weekdayJA(start))
	if title == "" {
		return label
	}
	return label + " " + title
}

func weekdayJA(t time.Time) string {
	return [...]string{"日", "月", "火", "水", "木", "金", "土"}[int(t.Weekday())]
}

// ------------------------------------------------------------------ 要約 ---

// premortemSystemPrompt は死因の洗い出しの指示。出力の「形」は premortemReportSchema が
// 縛るので、ここには「中身」の指示だけを書く。順位付けと件数の集計は premortem_rank.go の
// 仕事なので、「繰り返しを優先しろ」「件数を数えろ」の類は書かない（#658 の原則）。
func premortemSystemPrompt(few bool, pc premortemPromptContext) string {
	risks := "4〜8 個"
	if few {
		risks = "最大 3 個"
	}
	return premortemPromptPreamble(pc) + `あなたはアメリカンフットボールチームのコーチ補佐です。
次の試合は終わり、チームは負けました。あなたはその翌日に「なぜ負けたか」を振り返っています。
入力は Slack に投稿された「練習の投稿とその反省スレッド」と、そこに貼られた資料です。
[投稿] 行が投稿本文、[投稿・返信なし] 行は返信の付いていない投稿、その下の "- 名前: 本文" が
反省の書き込み、[リンク] [ファイル] 行が貼られた資料です。

# 全体の方針
- **入力に既に現れている兆候から**、試合で崩れる箇所を挙げる。入力に根拠の無い一般論
  （フィジカル不足・気持ちの問題・練習量）は挙げない。
- 一般論を書かない。読んだ人が今週の練習で何を変えるかを決められる具体性まで踏み込む。
- 次の語は、直後に具体的な動作（誰が・どの局面で・何を）が続かない限り使わない:
  意識する／徹底する／自信を持つ／コミュニケーション／連携／集中
- 良い／悪い のような評価語ではなく、観察された事実と、その原因を書く。
- 値の文字列に Slack の装飾記号（* や _）を入れない。

# risks（負け筋）
- 反省スレッドと資料から読み取れる負け筋を ` + risks + ` 挙げる。
- kind は死因の型。次のいずれかを必ず選ぶ:
  - execution: 練習でできていることが試合で出ない（実行の失敗）
  - preparation: 対策そのものが用意されていなかった（準備の欠落）
  - hypothesis: 相手・展開の事前の想定が外れた（上流の判断ミス）
  - matchup: 単純な個の力負け
  - adjust: 現場で起きていることを正しく把握できず、正しいカードを切り替えられなかった
- execution と adjust は混同しやすい。「できるはずのことができなかった」が execution、
  「やるべきことの選択を誤った・変えられなかった」が adjust。
- hypothesis と adjust は 2 段構え。想定が外れること自体（hypothesis）と、
  外れたと気づいて打ち手を変えられないこと（adjust）は別の死因として分けて挙げる。
- **hypothesis か adjust を最低 1 件は挙げる**。execution / preparation / matchup は
  「準備と実力」の話で、反省スレッドに書いてあるのはそればかりなので放っておくとそこにしか行かない。
  ただし根拠が無いなら挙げない（推測で埋めない）。この 2 つがぶつかったら根拠を優先する。
- key は負け筋を識別する短い英小文字のスラッグ（例: "ol_slide_late"）。plays から参照するので一意にする。
- title は 1 行の見出し。誰が・どの局面で・何が起きるかが分かる形にする。
- label は目次と凡例に出す 14 文字以内の短い名詞句（例: "3rd&long の被サック"）。title を要約したものにする。
- opponent は相手側の前提を 1 文で。**入力に根拠があるときだけ書く**。無ければ空文字。
  「相手は強い」「走を止められないと苦しい」のような、相手が誰でも成立する一般論は書かない。
  相手の情報は上乗せであって、premortem の土台ではない。材料が無いなら空文字のままでよく、
  埋めるために推測しない。
- opponent_quote は opponent の根拠になった入力の原文から 20〜40 文字をそのまま抜く
  （要約しない・言い換えない）。opponent が空なら空文字。**この引用が入力に見つからなければ
  opponent は破棄される**ので、必ず原文どおりに写す。
- scenario はどう崩れるかを 1〜2 文で。根拠になった観察を含める。
  opponent がある負け筋では、opponent に続けて読んで意味が通るように書く（「対してこちらは…」
  「こちらは…」）。ただし opponent が落ちても単体で文として成立させる。
- phase は崩れる局面（例: "3rd&long" "レッドゾーン" "前半の 1st down" "2 ミニッツ" "キッキング"）。
  特定できなければ空文字。
- unit は OF / DF / ST のいずれか。特定できなければ空文字。
- signal は「試合中に何が見えたら赤信号か」を 1 つ。誰が何を見て、何が起きたらどう切り替えるかまで書く。
  とくに kind が adjust のときは、サイドラインの誰が見るかを必ず含める。
- prevent には今週やることを動作で 1〜2 件。誰が・どの練習で・何を、まで書く。
- positions には関係するポジション（QB, WR, OL など）を入れる。特定できなければ空配列。
- quote には入力の原文から 20〜40 文字をそのまま抜く（要約しない・言い換えない）。

# plays（根拠になったプレー）
- 入力に現れたプレー投稿を、入力の並び順のまま 1 件ずつ挙げる。
- name は投稿されたプレー名をそのまま使う。
- risk_keys にはそのプレーが根拠になる risks の key を入れる。該当が無ければ空配列。
- @channel や @here を含む告知、「ナイスオフェンス！！」のような感想はプレーとして扱わない。`
}

// dropUnbackedOpponents は、根拠の裏が取れない相手軸を落とす。
//
// 相手名で照合しない。カレンダーのイベントタイトルは `#試合 vs A` 形式で、`#試合` を剥がして
// 残る `vs A` が Slack 上の呼び方（略称・愛称）と一致する保証が無い。名前を検索語にすると、
// 材料があるのに空振りして相手軸が消える。代わりに opponent_quote（入力からの逐語抜き）が
// corpus に在るかを見る（#698 決定 3）。
//
// 落ちても premortem は壊れない。相手の情報は上乗せであって土台ではないので、
// opponent が空になれば描画は scenario だけに戻る（#698 大原則）。
func dropUnbackedOpponents(risks []premortemRisk, corpus string) []premortemRisk {
	haystack := normalizeForQuoteMatch(corpus)
	for i := range risks {
		quote := normalizeForQuoteMatch(risks[i].OpponentQuote)
		if quote == "" || !strings.Contains(haystack, quote) {
			risks[i].Opponent = ""
			risks[i].OpponentQuote = ""
		}
	}
	return risks
}

// normalizeForQuoteMatch は逐語照合の前処理。空白・改行の差だけは吸収するが、それ以上は
// 緩めない（緩めると「だいたい似た文字列」が通ってしまい、検証にならない）。
func normalizeForQuoteMatch(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// premortemPromptPreamble は本文の前に置く文脈。何も引けなければ空文字を返し、
// プロンプトは #683 のときと同じ形になる（相手軸なしでも成立する）。
func premortemPromptPreamble(pc premortemPromptContext) string {
	lines := []string{}
	if pc.Game != "" {
		lines = append(lines, "- 次の試合: "+pc.Game)
	}
	if pc.Scope != "" {
		scope := "- この会話は #" + pc.Scope + " チャンネルのもの。ここがスコープを規定する。"
		if len(pc.Extras) > 0 {
			scope += "参考として " + strings.Join(pc.Extras, " ") + " も読んでいる。"
		}
		lines = append(lines, scope,
			"- チャンネルのスコープから、こちらの誰が相手の何とマッチアップするかを判断する。"+
				"例: #wr ならこちらのレシーバー陣と相手のセカンダリ、#o-tight ならこちらのラン攻撃と"+
				"相手のフロント 7、#defense ならこちらの守備と相手のオフェンス。"+
				"フットボールの一般的な知識で判断してよい。")
	}
	if len(lines) == 0 {
		return ""
	}
	return "# いま読んでいるもの\n" + strings.Join(lines, "\n") + "\n\n"
}

// summarizePremortem は全スレッドを 1 プロンプトにまとめて ChatGPT を呼ぶ。
// 分割が起きたかどうかを bool で返す（黙って質が落ちるのを防ぐため呼び出し側が通知する）。
func (bot Bot) summarizePremortem(ctx context.Context, job premortemJob, sink premortemSink, threads []playThread, resolve func(string) string, pc premortemPromptContext) (premortemSummary, error) {
	groups := splitThreadsForPrompt(threads, focusPromptRuneBudget)
	if len(groups) > 1 {
		// 分割そのものは黙って起こさない。名寄せ（#688）で収束はさせるが、
		// 1 回で読み切れなかった事実は読み手に見えるようにしておく。
		sink.Notice(fmt.Sprintf(
			"⚠️ 対象が多いため入力を %d 分割して読みました（重複した負け筋は名寄せしています）",
			len(groups)))
	}

	few := premortemFewTargets(job, threads)
	prompt := premortemSystemPrompt(few, pc)
	parts := make([]string, 0, len(groups))
	digest := premortemDigest{}
	structured := true
	// corpus は LLM に見せた平文そのもの。opponent_quote の照合先はここ（#698 決定 10）。
	// 元の slack.Message と照合すると、resolve 済みメンションや unfurl したリンク本文から
	// 引いた引用を「入力に無い」と誤判定する。
	corpus := &strings.Builder{}
	for _, group := range groups {
		rendered := renderPremortemThreads(group, resolve)
		corpus.WriteString(rendered)
		reply, err := bot.chat(ctx, ChatRequest{
			Model:  chatModelFocus,
			System: []string{prompt},
			User:   rendered,
			Schema: &ChatJSONSchema{Name: premortemReportSchemaName, Schema: premortemReportSchema},
		})
		if err != nil {
			return premortemSummary{}, err
		}
		content := strings.TrimSpace(reply)
		parts = append(parts, content)
		if !structured {
			continue
		}
		part := premortemDigest{}
		if err := json.Unmarshal([]byte(content), &part); err != nil {
			log.Printf("[premortem] structured output を受け取れませんでした, falling back to plain text: %v", err)
			structured = false
			continue
		}
		digest.Risks = append(digest.Risks, part.Risks...)
		digest.Plays = append(digest.Plays, part.Plays...)
	}

	summary := premortemSummary{Text: strings.Join(parts, "\n\n")}
	if !structured {
		return summary, nil
	}
	// 名寄せ・順位付けの前に相手軸の裏を取る。根拠の無い相手評をここで落としておけば、
	// 以降の層は「opponent があるなら根拠がある」を前提にできる。
	digest.Risks = dropUnbackedOpponents(digest.Risks, corpus.String())
	if len(groups) > 1 {
		// 塊ごとに出た重複を名寄せしてから数える（#688）。kind の重複回避も
		// 名寄せ後の 1 回で効かせる。1 塊なら 2 段目は呼ばない。
		digest = bot.mergePremortemDigest(ctx, digest)
	}
	report := rankRisks(digest, few)
	if len(report.Risks) == 0 {
		log.Printf("[premortem] digest has no risks, falling back to plain text")
		return summary, nil
	}
	if !hasJudgementKind(report.Risks) {
		// 判断系（hypothesis / adjust）が 1 件も無い。プロンプトの要求は満たしていないが、
		// 根拠が無いのに埋めさせるほうが害が大きいので強制も再問い合わせもしない（#683 決定 G-2）。
		log.Printf("[premortem] no judgement-kind risk adopted (hypothesis/adjust); posting as-is")
	}
	summary.Report = &report
	return summary, nil
}

// mergePremortemDigest は分割で重複して挙がった負け筋を名寄せする（#688）。
// 名寄せ後に rankRisks を回すので、kind の重複回避が全体に対して 1 回だけ効く。
func (bot Bot) mergePremortemDigest(ctx context.Context, d premortemDigest) premortemDigest {
	mapping := bot.mergeCandidateKeys(ctx, toMergeCandidates(d.Risks, func(risk premortemRisk) mergeCandidate {
		return mergeCandidate{Key: risk.Key, Title: risk.Title, Label: risk.Label}
	}))
	if len(mapping) == 0 {
		return d
	}
	d.Risks = keepCanonical(d.Risks, func(r premortemRisk) string { return r.Key }, mapping)
	for i := range d.Plays {
		d.Plays[i].RiskKeys = canonicalKeys(d.Plays[i].RiskKeys, mapping)
	}
	log.Printf("[premortem] 分割で重複した候補 %d 件を名寄せしました", len(mapping))
	return d
}

// premortemFewTargets は「対象が少ない」入力かどうか。focus と同じ閾値。
func premortemFewTargets(job premortemJob, threads []playThread) bool {
	if job.ThreadOnly {
		return true
	}
	plays, _, _ := countThreadKinds(threads)
	return plays < focusFewTargetsThreshold
}

func hasJudgementKind(risks []rankedRisk) bool {
	for _, r := range risks {
		for _, kind := range premortemJudgementKinds {
			if r.Kind == kind {
				return true
			}
		}
	}
	return false
}

// renderPremortemThreads はプロンプトに渡す平文へ整形する。focus の renderThreads に
// 加えて、貼られたリンクの unfurl とテキスト系ファイルの中身を載せる。相手チームの材料は
// 外部から取得できず、Slack に上がったものが全てなので、ここを落とすと hypothesis が立たない。
func renderPremortemThreads(threads []playThread, resolve func(string) string) string {
	buf := &strings.Builder{}
	for _, t := range threads {
		text := resolveMentions(strings.TrimSpace(t.Parent.Text), resolve)
		if len(t.Replies) == 0 {
			fmt.Fprintf(buf, "\n[投稿・返信なし] %s\n", text)
		} else {
			fmt.Fprintf(buf, "\n[投稿] %s\n", text)
		}
		for _, line := range messageMaterials(t.Parent) {
			fmt.Fprintf(buf, "  %s\n", line)
		}
		for _, r := range t.Replies {
			body := strings.TrimSpace(resolveMentions(r.Text, resolve))
			if body != "" {
				fmt.Fprintf(buf, "  - %s: %s\n", resolveUser(r.User, resolve), strings.ReplaceAll(body, "\n", " "))
			}
			for _, line := range messageMaterials(r) {
				fmt.Fprintf(buf, "  %s\n", line)
			}
		}
	}
	return buf.String()
}

// messageMaterials は 1 メッセージに付いた資料を行に起こす。
// - リンク: Slack の unfurl 結果（Attachments）。外部を自分で取りに行かなくても中身が読める
// - ファイル: テキスト系だけ本文が入る（Preview / PlainText）。画像・PDF は本文が無いので題名だけ
func messageMaterials(m slack.Message) []string {
	lines := []string{}
	for _, a := range m.Attachments {
		parts := trimStrings([]string{a.Title, a.Text})
		if body := joinNonEmpty(parts, " — "); body != "" {
			lines = append(lines, "[リンク] "+oneLine(body))
		}
	}
	for _, f := range m.Files {
		name := strings.TrimSpace(f.Title)
		if name == "" {
			name = strings.TrimSpace(f.Name)
		}
		body := strings.TrimSpace(f.PlainText)
		if body == "" {
			body = strings.TrimSpace(f.Preview)
		}
		switch {
		case name == "" && body == "":
			continue
		case body == "":
			lines = append(lines, fmt.Sprintf("[ファイル] %s（%s。本文は取得できません）", oneLine(name), f.Filetype))
		default:
			lines = append(lines, fmt.Sprintf("[ファイル] %s: %s", oneLine(name), oneLine(body)))
		}
	}
	return lines
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// ------------------------------------------------------------------ 投稿 ---

// premortemDoneText は受付メッセージの最終形。読んだ範囲を実数で残す。
func premortemDoneText(job premortemJob, threads []playThread, now time.Time, elapsed time.Duration) string {
	plays, _, replies := countThreadKinds(threads)
	if job.ThreadOnly {
		return fmt.Sprintf("✅ このスレッドの %d 件の返信を読みました（%s）", replies, focusElapsedLabel(elapsed))
	}
	return fmt.Sprintf("✅ %s の %d チャンネル / %d プレー / %d 件の反省を読みました（%s）",
		focusRangeLabel(job.focusJobFor(job.Channel), now), len(job.Sources), plays, replies,
		focusElapsedLabel(elapsed))
}

// premortemHeader は平文フォールバック用の見出し。構造化に失敗した経路でも射程は申告する
// （#697）。ここを抜かすと、最も質の落ちた出力だけが自分の射程を名乗らないことになる。
func premortemHeader(job premortemJob, threads []playThread, game string, now time.Time) string {
	head := "*" + premortemTitle(job, game, now) + "*"
	if notice := premortemScopeNotice(job); notice != "" {
		return head + "\n" + notice
	}
	return head
}
