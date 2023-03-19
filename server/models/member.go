package models

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"cloud.google.com/go/datastore"
)

type (
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
	MemberStatus string
)

var (
	memberCache = map[string]Member{}
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

func GetMemberInfoByCache(ctx context.Context, id string) (m Member, err error) {
	if m, ok := memberCache[id]; ok {
		return m, nil
	}
	memberCache, err = GetAllMembersAsDict(ctx)
	if err != nil {
		return m, err
	}
	if m, ok := memberCache[id]; !ok {
		return m, fmt.Errorf("not found for id:%v", id)
	} else {
		return m, nil
	}
}
