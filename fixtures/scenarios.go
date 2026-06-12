package fixtures

import (
	"fmt"
	"sort"
	"time"

	"github.com/triax/hub/server/models"
)

// registry は名前 → scenario ビルダーのレジストリ。
// scenario はビルダー関数として登録する（相対日付など実行時計算を含むため）。
var registry = map[string]func(now time.Time) Scenario{
	"default": defaultScenario,
}

// Names は登録済み scenario 名を返す（ソート済み）。
func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Resolve は名前リストから scenario を構築して合成する。
// 未知の名前はエラー。
func Resolve(now time.Time, names ...string) (Scenario, error) {
	scenarios := make([]Scenario, 0, len(names))
	for _, name := range names {
		build, ok := registry[name]
		if !ok {
			return Scenario{}, fmt.Errorf("unknown scenario %q (available: %v)", name, Names())
		}
		scenarios = append(scenarios, build(now))
	}
	return Compose(scenarios...)
}

// defaultScenario は全 env で必要な最小ベースライン。
//   - local-user.json の SlackID を持つ admin Member 1 件（自動ログインの本人）
//   - 直近・近未来の Event 3 件（home 画面が空にならないように。相対日付）
//
// いずれも他 entity を参照しないため dangling は発生しない。
func defaultScenario(now time.Time) Scenario {
	const localUserSlackID = "U9MD7M0NS" // server/filters/local-user.json の openid.sub

	admin := &models.Member{
		Status: models.MSActive,
	}
	admin.Slack.ID = localUserSlackID
	admin.Slack.TeamID = "T9LHPRHA6"
	admin.Slack.Name = "otiai10"
	admin.Slack.RealName = "Hiromu Ochiai"
	admin.Slack.IsAdmin = true
	admin.Slack.Profile.RealName = "Hiromu Ochiai"
	admin.Slack.Profile.DisplayName = "otiai10"
	admin.Slack.Profile.Title = "老害/Staff"

	entities := []Entity{
		NewEntity(MemberKey(localUserSlackID), admin),
	}

	eventDefs := []struct {
		id    string
		title string
		days  int
	}{
		{"fixture_event_practice_01", "#練習 第1回 春季練習", -7},
		{"fixture_event_practice_02", "#練習 第2回 春季練習", -3},
		{"fixture_event_game_01", "#試合 練習試合 vs Fixtures", 3},
		{"fixture_event_sponsor_01", "#sponsor スポンサー説明会", 7},
		{"fixture_event_sponsor_02", "#スポンサー 協賛企業交流会", 10},
	}
	for _, d := range eventDefs {
		start := now.AddDate(0, 0, d.days)
		ev := &models.Event{}
		ev.Google.ID = d.id
		ev.Google.Title = d.title
		ev.Google.StartTime = start.UnixMilli()
		ev.Google.EndTime = start.Add(3 * time.Hour).UnixMilli()
		entities = append(entities, NewEntity(EventKey(d.id), ev))
	}

	return Scenario{Name: "default", Entities: entities}
}
