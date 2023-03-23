package models

import (
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
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
)

var (
	EquipmentExceptionExpression = regexp.MustCompile("!\\(([^)]+)\\)")
)

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

func (equip Equip) ShouldBringFor(event Event) bool {
	if !equip.ForPractice && !equip.ForGame {
		return false
	}
	if event.IsGame() && equip.ForGame {
		return true
	}
	if !event.IsPractice() || !equip.ForPractice {
		return false
	}
	// 以下、練習用の特殊ケース
	match := EquipmentExceptionExpression.FindStringSubmatch(equip.Description)
	if len(match) < 2 {
		return equip.ForPractice
	}
	for _, exception := range strings.Split(match[1], ",") {
		if strings.Contains(event.Google.Title, exception) {
			return false
		}
	}
	return equip.ForPractice
}
