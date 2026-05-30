//go:build ignore

// テスト用シードデータをエミュレーターに投入するスクリプト
// 使い方: DATASTORE_EMULATOR_HOST=localhost:9206 go run seed_test_data.go

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/server/models"
)

func main() {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, os.Getenv("DATASTORE_PROJECT_ID"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// --- Member（local-user.json の SlackID で admin 権限を持つメンバー）---
	member := models.Member{}
	member.Slack.ID = "U9MD7M0NS"
	member.Slack.IsAdmin = true
	member.Slack.RealName = "Hiromu Ochiai (Test Admin)"
	member.Slack.Profile.Title = "Staff/Admin"
	member.Slack.Profile.RealName = "Hiromu Ochiai"
	member.Slack.Profile.DisplayName = "otiai10"
	memberKey := datastore.NameKey(models.KindMember, "U9MD7M0NS", nil)
	if _, err := client.Put(ctx, memberKey, &member); err != nil {
		log.Fatal("member:", err)
	}
	fmt.Println("✓ Member seeded")

	// --- Events（直近40日以内のイベント）---
	now := time.Now()
	eventDefs := []struct {
		id    string
		title string
		days  int
	}{
		{"event_practice_01", "第1回 春季練習", -7},
		{"event_practice_02", "第2回 春季練習", -3},
		{"event_game_01", "練習試合 vs XXXX", 3},
	}
	for _, def := range eventDefs {
		start := now.AddDate(0, 0, def.days)
		ev := models.Event{}
		ev.Google.ID = def.id
		ev.Google.Title = def.title
		ev.Google.StartTime = start.Unix() * 1000
		ev.Google.EndTime = start.Add(3 * time.Hour).Unix() * 1000
		key := datastore.NameKey(models.KindEvent, def.id, nil)
		if _, err := client.Put(ctx, key, &ev); err != nil {
			log.Fatal("event:", err)
		}
		fmt.Printf("✓ Event seeded: %s\n", def.title)
	}

	// --- TapingMenuItems ---
	menuItems := []models.TapingMenuItem{
		{Name: "足首（片足）", Price: 300, EstimatedRolls: 1.5, SortOrder: 1},
		{Name: "足首（両足）", Price: 500, EstimatedRolls: 3.0, SortOrder: 2},
		{Name: "靴上（両足）", Price: 400, EstimatedRolls: 2.0, SortOrder: 3},
		{Name: "膝（片方）", Price: 400, EstimatedRolls: 2.0, SortOrder: 4},
		{Name: "膝（両膝）", Price: 700, EstimatedRolls: 4.0, SortOrder: 5},
		{Name: "肩（片方）", Price: 300, EstimatedRolls: 1.5, SortOrder: 6},
		{Name: "肩（両肩）", Price: 500, EstimatedRolls: 3.0, SortOrder: 7},
		{Name: "肘（片方）", Price: 300, EstimatedRolls: 1.0, SortOrder: 8},
		{Name: "肘（両肘）", Price: 500, EstimatedRolls: 2.0, SortOrder: 9},
		{Name: "手首（片方）", Price: 200, EstimatedRolls: 0.5, SortOrder: 10},
		{Name: "手首（両方）", Price: 350, EstimatedRolls: 1.0, SortOrder: 11},
		{Name: "筋肉サポート１箇所", Price: 300, EstimatedRolls: 1.0, SortOrder: 12},
		{Name: "筋肉サポート２箇所", Price: 500, EstimatedRolls: 2.0, SortOrder: 13},
		{Name: "圧迫サポート（キネシオ）１箇所", Price: 400, EstimatedRolls: 1.0, SortOrder: 14},
		{Name: "圧迫サポート（キネシオ）２箇所", Price: 700, EstimatedRolls: 2.0, SortOrder: 15},
		{Name: "肉離れフル", Price: 1000, EstimatedRolls: 4.0, SortOrder: 16},
	}
	for i := range menuItems {
		key := datastore.IncompleteKey(models.KindTapingMenuItem, nil)
		newKey, err := client.Put(ctx, key, &menuItems[i])
		if err != nil {
			log.Fatal("menu item:", err)
		}
		fmt.Printf("✓ TapingMenuItem seeded: %s (id=%d)\n", menuItems[i].Name, newKey.ID)
	}

	fmt.Println("\n✅ Seed complete!")
}
