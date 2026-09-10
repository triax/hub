package models

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

type (
	Event struct {
		Google GoogleEvent `json:"google"`

		ParticipationsJSONString string `json:"participations_json_str" datastore:",noindex"`
	}

	Participations map[string]Participation

	Participation struct {
		Type   ParticipationType      `json:"type"`
		Params map[string]interface{} `json:"params"`
	}

	ParticipationType string
)

const (
	PTJoin       ParticipationType = "join"
	PTJoinLate   ParticipationType = "join_late"
	PTLeaveEarly ParticipationType = "leave_early"
	PTAbsent     ParticipationType = "absent"
	PTUnanswered ParticipationType = "unanswered"
)

type (
	ReminderType string
)

const (
	RTRSVP      ReminderType = "rsvp"
	RTFinalCall ReminderType = "final_call"
	RTCondition ReminderType = "condition"
	RTEquipment ReminderType = "equipment"
)

type (
	EventTag string
)

const (
	ETPractice EventTag = "練習"
	ETGame     EventTag = "試合"
	ETIgnore   EventTag = "ignore"
	ETMeeting  EventTag = "meeting"
	ETEvent    EventTag = "event"
	ETSponsor  EventTag = "sponsor"
	ETUnkonwn  EventTag = "UNKNOWN"
)

var (
	EventExpressionPractice = regexp.MustCompile("[＃#]練習")
	EventExpressionGame     = regexp.MustCompile("[＃#]試合")
	EventExpressionIgnore   = regexp.MustCompile("[＃#]ignore")
	EventExpressionEvent    = regexp.MustCompile("[＃#]event")
	EventExpressionMeeting  = regexp.MustCompile("[＃#](meeting|mtg)")
	EventExpressionSponsor  = regexp.MustCompile("[＃#](sponsor|スポンサー)")
)

func (pt ParticipationType) String() string {
	switch pt {
	case PTJoin:
		return "出席"
	case PTJoinLate:
		return "遅参"
	case PTLeaveEarly:
		return "早退"
	case PTAbsent:
		return "欠席"
	case PTUnanswered:
		return "未回答"
	default:
		return "不明"
	}
}

func (e Event) Participations() (Participations, error) {
	p := Participations{}
	err := json.NewDecoder(strings.NewReader(e.ParticipationsJSONString)).Decode(&p)
	return p, err
}

func (e Event) IsPractice() bool {
	return e.HasTag(ETPractice)
}

func (e Event) IsGame() bool {
	return e.HasTag(ETGame)
}

func (e Event) ShouldSkipReminders(rt ReminderType) bool {
	tags := e.Tags()
	// #ignore は最優先: 含まれていれば全リマインダを skip する（明示オプトアウト）
	for _, t := range tags {
		if t == ETIgnore {
			return true
		}
	}
	// most-permissive: いずれかのタグが当該リマインダの送信を望むなら送信する
	// （= 全タグが skip と判断したときのみ skip する）
	for _, t := range tags {
		if !tagSkipsReminder(t, rt) {
			return false
		}
	}
	return true
}

// tagSkipsReminder は単一タグが当該リマインダ種別を skip すべきかを返す。
func tagSkipsReminder(t EventTag, rt ReminderType) bool {
	switch t {
	case ETIgnore, ETMeeting:
		return true // ignore / meeting は全リマインダを skip
	case ETEvent, ETSponsor:
		return rt != RTRSVP // event / sponsor は RSVP 以外を skip
	default:
		return false // 練習 / 試合 / UNKNOWN は skip しない
	}
}

// tagDefs はタグ判定の唯一の定義（判定順序込み）。Tags() / HasTag() はこれを回す。
// クライアント TriaxEvent.ts の TAG_PATTERNS と順序・正規表現を一致させること。
var tagDefs = []struct {
	tag EventTag
	re  *regexp.Regexp
}{
	{ETPractice, EventExpressionPractice},
	{ETGame, EventExpressionGame},
	{ETIgnore, EventExpressionIgnore},
	{ETMeeting, EventExpressionMeeting},
	{ETEvent, EventExpressionEvent},
	{ETSponsor, EventExpressionSponsor},
}

// Tags はタイトルに含まれる全てのタグを返す（複数タグ対応）。
// 該当タグが無ければ ETUnkonwn ひとつを返す。
func (e Event) Tags() []EventTag {
	tags := []EventTag{}
	for _, d := range tagDefs {
		if d.re.MatchString(e.Google.Title) {
			tags = append(tags, d.tag)
		}
	}
	if len(tags) == 0 {
		tags = append(tags, ETUnkonwn)
	}
	return tags
}

// HasTag は指定タグがタイトルに含まれるかを返す。
// Tags() と一貫させるため、タグ無し時の ETUnkonwn も正しく判定できる。
func (e Event) HasTag(t EventTag) bool {
	return slices.Contains(e.Tags(), t)
}

func (t ParticipationType) JoinAnyhow() bool {
	return t == PTJoin || t == PTJoinLate || t == PTLeaveEarly
}

func (t ParticipationType) Unanswered() bool {
	return t == "" || t == PTUnanswered
}

// timeboud[0] == いつから
// timeboud[1] == いつまで
func FindEventsBetween(ctx context.Context, timebound ...time.Time) (events []Event, err error) {

	if len(timebound) == 0 {
		timebound = []time.Time{time.Now()}
	}
	if len(timebound) == 1 {
		timebound = append(timebound, timebound[0].Add(24*time.Hour))
	}
	from := timebound[0]
	to := timebound[1]
	if !from.Before(to) {
		return nil, fmt.Errorf("invalid time-bound")
	}

	query := datastore.NewQuery(KindEvent)
	if !from.IsZero() {
		query = query.Filter("Google.StartTime >=", from.Unix()*1000)
	}
	if !to.IsZero() {
		query = query.Filter("Google.StartTime <", to.Unix()*1000)
	}
	query = query.Order("-Google.StartTime")
	query = query.Limit(10)

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, fmt.Errorf("datastore client initiation error: %v", err)
	}
	defer client.Close()

	if _, err = client.GetAll(ctx, query, &events); err != nil {
		return nil, fmt.Errorf("datastore query error: %v", err)
	}
	return
}

// UpcomingGameLookahead は FindUpcomingGame が既定で先を見る幅。premortem（#683）が
// 「次の試合」を引くのに使う。長すぎると当面関係ない試合を掴むので 2 週間に切る。
const UpcomingGameLookahead = 14 * 24 * time.Hour

// FindUpcomingGame は from から within の間で最も近い「#試合」を 1 件返す。
//
// FindEventsBetween は使えない。あちらは Order("-Google.StartTime") かつ Limit(10) なので、
// 窓を広く取ると返るのは「最も遠い 10 件」で、最も近い試合が黙って落ちる。既存の
// 呼び出し元はいずれも窓が 1 日程度で events[0] が成立しているだけなので、
// あちらの並び順・件数上限は変えずに専用のクエリを立てる（#683）。
//
// 「#試合」は Google.Title の正規表現判定でインデックスできないため、Datastore 側では
// 絞れない。開始時刻の昇順・件数上限なしで窓ぶんを読み、Go 側で最初の 1 件を採る。
func FindUpcomingGame(ctx context.Context, from time.Time, within time.Duration) (Event, bool, error) {
	if within <= 0 {
		return Event{}, false, fmt.Errorf("invalid lookahead: %v", within)
	}

	// 昇順。近い順に読むので、FindEventsBetween の「遠い 10 件しか返らない」問題が
	// 構造的に起きない。Limit も置かない（窓の手前に非試合イベントが何件詰まっていても
	// 取りこぼさないため。14 日ぶんなら数十件で収まる）。
	query := datastore.NewQuery(KindEvent).
		Filter("Google.StartTime >=", from.Unix()*1000).
		Filter("Google.StartTime <", from.Add(within).Unix()*1000).
		Order("Google.StartTime")

	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return Event{}, false, fmt.Errorf("datastore client initiation error: %v", err)
	}
	defer client.Close()

	events := []Event{}
	if _, err := client.GetAll(ctx, query, &events); err != nil {
		return Event{}, false, fmt.Errorf("datastore query error: %v", err)
	}
	event, ok := pickUpcomingGame(events)
	return event, ok, nil
}

// pickUpcomingGame は開始時刻の昇順に並んだイベント列から、対象試合を 1 件選ぶ規則。
// Datastore に触らないので、エミュレーター無しで単体テストできる（#683）。
// #ignore は明示オプトアウトなので、#試合 が付いていても対象にしない。
func pickUpcomingGame(events []Event) (Event, bool) {
	for _, e := range events {
		if e.IsGame() && !e.HasTag(ETIgnore) {
			return e, true
		}
	}
	return Event{}, false
}
