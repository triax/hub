package models

const KindMember = "Member"

type (
	Member struct {
		Slack SlackMember `json:"slack"`
	}
)
