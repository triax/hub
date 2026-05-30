package models

import "cloud.google.com/go/datastore"

type TapingMenuItem struct {
	ID             int64          `json:"id" datastore:"-"`
	Key            *datastore.Key `datastore:"__key__"`
	Name           string         `json:"name"`
	Price          int            `json:"price"`
	EstimatedRolls float64        `json:"estimated_rolls"`
	Notes          string         `json:"notes"`
	SortOrder      int            `json:"sort_order"`
	Disabled       bool           `json:"disabled"`
}

// Taping は1部位=1エンティティ。
// NameKey: memberID + "_" + eventID + "_" + menuItemID
// → client.Put が自動 upsert になり、再申請は差分削除＋Put で実現する。
type Taping struct {
	Key            *datastore.Key `datastore:"__key__"`
	MemberID       string         `json:"member_id"`
	EventID        string         `json:"event_id"`
	MenuItemID     int64          `json:"menu_item_id"`
	MenuItemName   string         `json:"menu_item_name"`
	Price          int            `json:"price"`
	EstimatedRolls float64        `json:"estimated_rolls"`
	RequestedAt    int64          `json:"requested_at"`
}
