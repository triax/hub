package models

import (
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
	if event.IsGame() {
		return equip.ForGame
	}
	if event.IsPractice() {
		return equip.ForPractice
	}
	return false
}
