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

type (
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
