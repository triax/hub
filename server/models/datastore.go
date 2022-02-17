package models

import (
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
