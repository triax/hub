package models

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

const (
	KindMember  = "Member"
	KindEvent   = "Event"
	KindEquip   = "Equip"
	KindCustody = "Custody"
)

type (
	Equip struct {
		ID          int64          `json:"id" datastore:"-"`
		Key         *datastore.Key `datastore:"__key__"`
		Name        string         `json:"name"`
		ForPractice bool           `json:"for_practice"`
		ForGame     bool           `json:"for_game"`
		Description string         `json:"description"`
		History     []Custody      `json:"history" datastore:"-"`
	}

	Custody struct {
		Key       *datastore.Key `datastore:"__key__"`
		MemberID  string         `json:"member_id"`
		Timestamp int64          `json:"ts"`
		Comment   string         `json:"comment"`
	}

	Member struct {
		Team  SlackTeam `json:"team"`
		Slack SlackUser `json:"slack"`

		// Status メンバーの（退部済み以外の）参加状態
		Status MemberStatus `json:"status"`

		// Number 背番号
		// 背番号ゼロに対応するためにポインタを使わざるを得ない.
		// おおおか、許すまじ
		Number *int `json:"number"`
	}

	Event struct {
		Google GoogleEvent `json:"google"`

		ParticipationsJSONString string `json:"participations_json_str" datastore:",noindex"`
	}

	Participations map[string]Participation

	Participation struct {
		Type    ParticipationType      `json:"type"`
		Params  map[string]interface{} `json:"params"`
		Name    string                 `json:"name"`
		Picture string                 `json:"picture"`
		Title   string                 `json:"title"`
	}

	ParticipationType string
	MemberStatus      string
)

const (
	PTJoin       ParticipationType = "join"
	PTJoinLate   ParticipationType = "join_late"
	PTLeaveEarly ParticipationType = "leave_early"
	PTAbsent     ParticipationType = "absent"
	PTUnanswered ParticipationType = "unanswered"
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

const (
	// MSActive 通常のメンバー. 出欠回答必須
	MSActive MemberStatus = "active"
	// MSLimited 部分的参加のメンバー. 出欠回答不要
	MSLimited MemberStatus = "limited"
	// MSInactive 休眠メンバー. 出欠回答不要
	MSInactive MemberStatus = "inactive"
	// MSDeleted 退部済みメンバー.
	// Member.Statusでは管理せず、Member.Slack.Deletedを使うため、使わないはず.
	MSDeleted MemberStatus = "deleted"
)

// IsFieldMismatch ...
// datastoreのGet系メソッド利用時において、
// datastore側で存在するフィールドを、struct側が持っていない場合、
// ErrFieldMismatchが起きるが、これはdataのマイグレーション上めんどくさいので、
// このエラーだけは無視したいことが多々ある。
// @See
// 	- https://github.com/googleapis/google-cloud-go/issues/913
//	- https://pkg.go.dev/cloud.google.com/go/datastore#ErrFieldMismatch
func IsFiledMismatch(err error) bool {
	_, ok := err.(*datastore.ErrFieldMismatch)
	return ok
}

func (m Member) Name() string {
	if m.Slack.RealName != "" {
		return m.Slack.RealName
	}
	if m.Slack.Profile.RealName != "" {
		return m.Slack.Profile.RealName
	}
	return m.Slack.Profile.DisplayName
}

// regexp.Compileのコストを減らしたい
var onMemRoleExpCache = map[string]*regexp.Regexp{}

func (m Member) IsMemberOf(roles ...string) (yes bool, role string, err error) {
	for _, r := range roles {
		exp, ok := onMemRoleExpCache[r]
		if !ok {
			exp, err = regexp.Compile("(?i)" + r)
			if err != nil {
				return false, "", err
			}
			onMemRoleExpCache[r] = exp
		}
		if exp.MatchString(m.Slack.Profile.Title) {
			return true, r, nil
		}
	}
	return false, "", nil
}

func (m Member) IsExpectedToRSVP() bool {
	if m.Status == MSDeleted || m.Status == MSLimited || m.Status == MSInactive {
		return false
	}
	return true
}

func (e Event) Participations() (Participations, error) {
	p := Participations{}
	err := json.NewDecoder(strings.NewReader(e.ParticipationsJSONString)).Decode(&p)
	return p, err
}

func (e Event) IsPractice() bool {
	return regexp.MustCompile("[＃#]練習").MatchString(e.Google.Title)
}

func (e Event) IsGame() bool {
	return regexp.MustCompile("[＃#]試合").MatchString(e.Google.Title)
}

func (e Event) ShouldSkipReminders() bool {
	return regexp.MustCompile("(?i)[＃#]ignore$").MatchString(e.Google.Title)
}

func (t ParticipationType) JoinAnyhow() bool {
	return t == PTJoin || t == PTJoinLate || t == PTLeaveEarly
}

func (t ParticipationType) Unanswered() bool {
	return t == "" || t == PTUnanswered
}

func (equip Equip) NeedsCharge() bool {
	return strings.HasPrefix(equip.Name, "ビデオ")
}

func (equip Equip) HasBeenUpdatedSince(t time.Time) bool {
	if len(equip.History) == 0 {
		return false
	}
	lastUpdated := time.Unix(equip.History[0].Timestamp/1000, 0)
	return lastUpdated.After(t)
}

// Accessor methods
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
		query.Filter("Google.StartTime >=", from.Unix()*1000)
	}
	if !to.IsZero() {
		query.Filter("Google.StartTime <", to.Unix()*1000)
	}
	query.Order("Google.StartTime")
	// query.Limit(1)

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

func GetAllMembers(ctx context.Context) ([]Member, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialized datastore client: %v", err)
	}
	defer client.Close()
	members := []Member{}
	query := datastore.NewQuery(KindMember)
	query = query.Filter("Slack.Deleted =", false)
	if _, err := client.GetAll(ctx, query, &members); err != nil && !IsFiledMismatch(err) {
		return nil, fmt.Errorf("failed to execute datastore query: %v", err)
	}
	return members, nil
}

func GetAllMembersAsDict(ctx context.Context) (map[string]Member, error) {
	members, err := GetAllMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to GetAllMembers: %v", err)
	}
	return MembersToDict(members), nil
}

func MembersToDict(members []Member) map[string]Member {
	dict := map[string]Member{}
	for _, m := range members {
		dict[m.Slack.ID] = m
	}
	return dict
}
