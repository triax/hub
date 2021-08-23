package models

import "time"

const (
	KindMember = "Member"
	KindEvent  = "Event"
)

const (
	PTJoin       ParticipationType = "join"
	PTJoinLate   ParticipationType = "join_late"
	PTLeaveEarly ParticipationType = "leave_early"
	PTAbsent     ParticipationType = "absent"
)

type (
	Member struct {
		Slack SlackMember `json:"slack"`
	}
	Event struct {
		Google GoogleEvent `json:"google"`

		// Participants Participations `json:"participations"`
		ParticipationsJSONString string `json:"participations_json_str"`

		StartDate time.Time `json:"start_date"` // なくてよい
		StartTime int64     `json:"start_time"` // なくてよい
		EndTime   int64     `json:"end_time"`   // なくてよい
	}

	Participations map[string]Participation

	Participation struct {
		Type    ParticipationType      `json:"type"`
		Name    string                 `json:"name"`
		Picture string                 `json:"picture"`
		Params  map[string]interface{} `json:"params"`
	}

	ParticipationType string
)
