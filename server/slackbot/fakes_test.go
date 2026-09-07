package slackbot

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/otiai10/openaigo"
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

// applyMsgOptions は MsgOption を実際に適用し、Slack へ送られる値を覗く。
func applyMsgOptions(channel string, options ...slack.MsgOption) url.Values {
	_, values, err := slack.UnsafeApplyMsgOptions("token", channel, "http://localhost/", options...)
	if err != nil {
		panic(err)
	}
	return values
}

type fakeSlackAPI struct {
	mu sync.Mutex

	// conversations.history のページ（呼ばれた順に返す）
	historyPages  []*slack.GetConversationHistoryResponse
	historyCalls  int
	historyParams []slack.GetConversationHistoryParameters

	// conversations.replies のページ（thread_ts ごと）
	repliesPages  map[string][][]slack.Message
	repliesIndex  map[string]int
	repliesParams []slack.GetConversationRepliesParameters

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
func (f *fakeSlackAPI) GetUserInfo(string) (*slack.User, error)                { return nil, nil }
func (f *fakeSlackAPI) GetReactions(slack.ItemRef, slack.GetReactionsParameters) (slack.ReactedItem, error) {
	return slack.ReactedItem{}, nil
}
func (f *fakeSlackAPI) GetConversations(*slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	return nil, "", nil
}
func (f *fakeSlackAPI) GetConversationInfo(*slack.GetConversationInfoInput) (*slack.Channel, error) {
	return nil, nil
}
func (f *fakeSlackAPI) OpenConversation(*slack.OpenConversationParameters) (*slack.Channel, bool, bool, error) {
	return nil, false, false, nil
}

type fakeChatGPT struct {
	requests []openaigo.ChatRequest
	reply    string
	err      error
}

func (f *fakeChatGPT) Chat(_ context.Context, req openaigo.ChatRequest) (openaigo.ChatCompletionResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return openaigo.ChatCompletionResponse{}, f.err
	}
	return openaigo.ChatCompletionResponse{
		Choices: []openaigo.Choice{{Message: openaigo.Message{Role: "assistant", Content: f.reply}}},
	}, nil
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
