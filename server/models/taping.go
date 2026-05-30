package models

import "cloud.google.com/go/datastore"

// TapeItem はテープ素材のマスタ（ホワイト、キネシオ等）。
type TapeItem struct {
	ID        int64          `json:"id" datastore:"-"`
	Key       *datastore.Key `datastore:"__key__"`
	Name      string         `json:"name"`
	SortOrder int            `json:"sort_order"`
	Disabled  bool           `json:"disabled"`
}

// TapeUsage は施術1件で使用するテープの種類と量。
type TapeUsage struct {
	TapeItemID   int64   `json:"tape_item_id"`
	TapeItemName string  `json:"tape_item_name"` // スナップショット
	Quantity     float64 `json:"quantity"`
}

type TapingMenuItem struct {
	ID             int64          `json:"id" datastore:"-"`
	Key            *datastore.Key `datastore:"__key__"`
	Name           string         `json:"name"`
	Price          int            `json:"price"`
	Notes          string         `json:"notes"`
	TapeUsagesJSON string         `json:"-" datastore:",noindex"` // JSON: []TapeUsage
	TapeUsages     []TapeUsage    `json:"tape_usages" datastore:"-"`
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
	TapeUsagesJSON string         `json:"-" datastore:",noindex"` // JSON: []TapeUsage（申請時スナップショット）
	TapeUsages     []TapeUsage    `json:"tape_usages" datastore:"-"`
	RequestedAt    int64          `json:"requested_at"`
}
