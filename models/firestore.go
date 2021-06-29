package models

const (
	KindMember = "Member"
	KindEvent  = "Event"
)

type (
	Member struct {
		Slack SlackMember `json:"slack"`
	}
	Event struct {
		Google GoogleEvent `json:"google"`
	}
)
