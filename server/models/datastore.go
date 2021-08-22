package models

import "time"

const (
	KindMember = "Member"
	KindEvent  = "Event"
)

const (
	PTJoin      PariticipationType = "join"
	PTJoinLate  PariticipationType = "join_late"
	PTDropEarly PariticipationType = "drop_early"
	PTAbsent    PariticipationType = "absent"
)

type (
	Member struct {
		Slack SlackMember `json:"slack"`
	}
	Event struct {
		Google GoogleEvent `json:"google"`

		Participants map[string]Pariticipation `json:"participations"`

		StartDate time.Time `json:"start_date"` // なくてよい
		StartTime int64     `json:"start_time"` // なくてよい
		EndTime   int64     `json:"end_time"`   // なくてよい
	}

	Pariticipation struct {
		Type   PariticipationType `json:"type"`
		Member SlackMember        `json:"member"`
	}

	PariticipationType string
)
