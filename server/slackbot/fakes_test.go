package slackbot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// ---- テスト用フェイク ------------------------------------------------------
// 実 Slack / 実 OpenAI / 実 Cloud Tasks は一切叩かない。

type sentMessage struct {
	Channel   string
	Timestamp string // UpdateMessage のときだけ埋まる
	Values    url.Values
}

func (s sentMessage) Text() string      { return s.Values.Get("text") }
func (s sentMessage) ThreadTS() string  { return s.Values.Get("thread_ts") }
func (s sentMessage) Broadcast() string { return s.Values.Get("reply_broadcast") }

// Blocks は blocks パラメータ（JSON 文字列）を素の map に戻す。
// Block Kit の構造（種類・並び・件数）をそのまま検査するため、型は付けない。
func (s sentMessage) Blocks() []map[string]any {
	raw := s.Values.Get("blocks")
	if raw == "" {
		return nil
	}
	blocks := []map[string]any{}
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		panic(err)
	}
	return blocks
}

// BlockTypes は blocks の並びを type だけの列にする（`header,context,…`）。
func (s sentMessage) BlockTypes() []string {
	types := []string{}
	for _, b := range s.Blocks() {
		t, _ := b["type"].(string)
		types = append(types, t)
	}
	return types
}

// 捕捉用のサーバとクライアントはテストバイナリで 1 組だけ立てる（メッセージごとに
// listener を張ると投稿数ぶんソケットを作ることになる）。捕捉した値はチャネルで
// 受け渡し、送信側とハンドラの間の同期も兼ねる。
var (
	captureOnce   sync.Once
	captureClient *slack.Client
	captureForm   = make(chan url.Values, 1)
	captureMu     sync.Mutex
)

// applyMsgOptions は MsgOption を実際に適用し、Slack へ送られる値を覗く。
//
// slack.UnsafeApplyMsgOptions は sendConfig.values しか返さず、blocks は
// formSender が組み立てる段階で初めて values に載る（chat.go の
// formSender.BuildRequestContext）。MsgOption の引数型は unexported なので
// テストから直接組み立てることもできない。そこで localhost の httptest サーバへ
// 実クライアントで 1 回 POST し、送信フォームをそのまま覗く。
// 外部通信は発生しない（同一プロセス内のループバックのみ）。
func applyMsgOptions(channel string, options ...slack.MsgOption) url.Values {
	captureMu.Lock()
	defer captureMu.Unlock()

	captureOnce.Do(func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				panic(err)
			}
			captureForm <- r.PostForm
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true,"channel":"C1","ts":"1.000000"}`)
		}))
		captureClient = slack.New("token", slack.OptionAPIURL(srv.URL+"/"))
	})

	if _, _, err := captureClient.PostMessage(channel, options...); err != nil {
		panic(err)
	}
	return <-captureForm
}

type fakeSlackAPI struct {
	mu sync.Mutex

	// conversations.history のページ（呼ばれた順に返す）
	historyPages  []*slack.GetConversationHistoryResponse
	historyCalls  int
	historyParams []slack.GetConversationHistoryParameters

	// historyByChannel / historyErrByChannel は premortem の複数チャンネル収集用。
	// 設定されていればチャンネルごとに引き、無ければ従来どおり historyPages を順に返す。
	historyByChannel    map[string]*slack.GetConversationHistoryResponse
	historyErrByChannel map[string]error

	// conversations.replies のページ（thread_ts ごと）
	repliesPages  map[string][][]slack.Message
	repliesIndex  map[string]int
	repliesParams []slack.GetConversationRepliesParameters

	// conversations.list が返すチャンネル（翻訳先の解決に使う）
	conversations []slack.Channel
	// conversations.info が返すチャンネル（翻訳元の名前を決める）
	channelInfo *slack.Channel

	posted   []sentMessage
	updated  []sentMessage
	added    []string
	removed  []string
	postedTS int

	postErr error
}

func newFakeSlackAPI() *fakeSlackAPI {
	return &fakeSlackAPI{
		repliesPages: map[string][][]slack.Message{},
		repliesIndex: map[string]int{},
	}
}

func (f *fakeSlackAPI) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.postErr != nil {
		return "", "", f.postErr
	}
	f.postedTS++
	f.posted = append(f.posted, sentMessage{Channel: channelID, Values: applyMsgOptions(channelID, options...)})
	return channelID, fmt.Sprintf("900.%06d", f.postedTS), nil
}

func (f *fakeSlackAPI) UpdateMessage(channelID, timestamp string, options ...slack.MsgOption) (string, string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, sentMessage{
		Channel: channelID, Timestamp: timestamp, Values: applyMsgOptions(channelID, options...),
	})
	return channelID, timestamp, "", nil
}

func (f *fakeSlackAPI) AddReaction(name string, _ slack.ItemRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.added = append(f.added, name)
	return nil
}

func (f *fakeSlackAPI) RemoveReaction(name string, _ slack.ItemRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, name)
	return nil
}

func (f *fakeSlackAPI) GetConversationHistory(params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.historyParams = append(f.historyParams, *params)
	if f.historyErrByChannel != nil {
		if err, ok := f.historyErrByChannel[params.ChannelID]; ok {
			return nil, err
		}
	}
	if f.historyByChannel != nil {
		if page, ok := f.historyByChannel[params.ChannelID]; ok {
			return page, nil
		}
		return &slack.GetConversationHistoryResponse{}, nil
	}
	if f.historyCalls >= len(f.historyPages) {
		return &slack.GetConversationHistoryResponse{}, nil
	}
	page := f.historyPages[f.historyCalls]
	f.historyCalls++
	return page, nil
}

func (f *fakeSlackAPI) GetConversationReplies(params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repliesParams = append(f.repliesParams, *params)
	pages := f.repliesPages[params.Timestamp]
	idx := f.repliesIndex[params.Timestamp]
	if idx >= len(pages) {
		return nil, false, "", nil
	}
	f.repliesIndex[params.Timestamp] = idx + 1
	next := ""
	if idx+1 < len(pages) {
		next = fmt.Sprintf("cursor-%d", idx+1)
	}
	return pages[idx], next != "", next, nil
}

// 以下は本テストでは使わないが SlackAPI interface を満たすために必要。
func (f *fakeSlackAPI) GetUsers(...slack.GetUsersOption) ([]slack.User, error) { return nil, nil }
func (f *fakeSlackAPI) GetUserInfo(string) (*slack.User, error)                { return &slack.User{}, nil }
func (f *fakeSlackAPI) GetReactions(slack.ItemRef, slack.GetReactionsParameters) (slack.ReactedItem, error) {
	return slack.ReactedItem{}, nil
}
func (f *fakeSlackAPI) GetConversations(*slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	return f.conversations, "", nil
}

func (f *fakeSlackAPI) GetConversationInfo(*slack.GetConversationInfoInput) (*slack.Channel, error) {
	if f.channelInfo == nil {
		return nil, fmt.Errorf("channel not found")
	}
	return f.channelInfo, nil
}

// OpenConversation は DM チャンネルを開く。「ありがとう」コマンドが戻り値の ID を
// そのまま PostMessage に渡すので、nil ではなく実体を返す。
func (f *fakeSlackAPI) OpenConversation(*slack.OpenConversationParameters) (*slack.Channel, bool, bool, error) {
	ch := &slack.Channel{}
	ch.ID = "D1"
	return ch, false, false, nil
}

type fakeChatGPT struct {
	requests []ChatRequest
	reply    string
	// replies は呼び出しごとに違う応答を返したいとき用（分割 + 2 段目 reduce の検証）。
	// 設定されていれば先頭から順に消費し、尽きたら reply に戻る。
	replies []string
	err     error
}

func (f *fakeChatGPT) Chat(_ context.Context, req ChatRequest) (string, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return "", f.err
	}
	if len(f.replies) > 0 {
		reply := f.replies[0]
		f.replies = f.replies[1:]
		return reply, nil
	}
	return f.reply, nil
}

type fakeEnqueuer struct {
	mu      sync.Mutex
	names   []string
	uris    []string
	bodies  []string
	err     error
	enqueue chan struct{}
}

func newFakeEnqueuer() *fakeEnqueuer {
	return &fakeEnqueuer{enqueue: make(chan struct{}, 8)}
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, name, relativeURI string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names = append(f.names, name)
	f.uris = append(f.uris, relativeURI)
	f.bodies = append(f.bodies, string(payload))
	select {
	case f.enqueue <- struct{}{}:
	default:
	}
	return f.err
}

func (f *fakeEnqueuer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.names)
}

// ---- メッセージ組み立てヘルパ ----------------------------------------------

func parentMsg(ts, text string, replyCount int) slack.Message {
	m := slack.Message{}
	m.Timestamp = ts
	m.Text = text
	m.ReplyCount = replyCount
	return m
}

func replyMsg(ts, user, text string) slack.Message {
	m := slack.Message{}
	m.Timestamp = ts
	m.User = user
	m.Text = text
	return m
}

func botMsg(ts, text string) slack.Message {
	m := slack.Message{}
	m.Timestamp = ts
	m.Text = text
	m.BotID = "B0BOT"
	m.SubType = "bot_message"
	return m
}

func historyPage(nextCursor string, msgs ...slack.Message) *slack.GetConversationHistoryResponse {
	res := &slack.GetConversationHistoryResponse{Messages: msgs}
	res.ResponseMetaData.NextCursor = nextCursor
	res.HasMore = nextCursor != ""
	return res
}

func texts(msgs []slack.Message) []string {
	out := []string{}
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

func joined(ss []string) string { return strings.Join(ss, ",") }

func mentionEvent(text, channel, ts, threadTS string) slackevents.AppMentionEvent {
	return slackevents.AppMentionEvent{
		Type: "app_mention", Text: text, Channel: channel,
		TimeStamp: ts, ThreadTimeStamp: threadTS,
	}
}
