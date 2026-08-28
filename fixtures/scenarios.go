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
	"default":  defaultScenario,
	"taping":   tapingScenario,
	"position": positionScenario,
	"equips":   equipsScenario,
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
		{"fixture_event_practice_sponsor_01", "#練習 #sponsor 合同練習＆協賛社見学", 5},
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

// tapingScenario はテーピング機能（/taping, /taping/request, /taping/master,
// /events/{id}/taping）の検証に必要な最小データ。
//
// default とは entity が一切重ならないので、単体でも default との合成でも成立する
// （全 scenario が単体で Validate を通ることは fixtures_test.go の不変条件）。
// そのため参照する Member / Event もこの scenario が自前で持つ。
//
// 表示順の検証（Issue #632）が成立するよう、意図的に次を満たしている:
//   - TapingMenuItem / TapeItem とも **ID の昇順と日本語名の昇順が一致しない**
//     （ソート未実装なら「登録順・キー順」で並び、実装後と見分けがつく）
//   - SortOrder はすべて 0（PROD と同じ状態。表示順は名前でしか決まらない）
//   - ひらがな・カタカナ・漢字が混在する
//   - 2 名がそれぞれ 2 件以上リクエスト済み（メンバー間の相対順序を比較できる）
func tapingScenario(now time.Time) Scenario {
	const (
		playerASlackID = "UFIXTUREPLAYER1"
		playerBSlackID = "UFIXTUREPLAYER2"
		targetEventID  = "fixture_event_taping_01"
	)

	entities := []Entity{}

	// --- 検証用メンバー（メンバー間で項目の相対順序を見比べるために2名） ---
	playerDefs := []struct {
		id, name, realName, displayName, title string
	}{
		{playerASlackID, "fixture-player-1", "Fixture Player 1", "ふぃくすちゃ一郎", "RB"},
		{playerBSlackID, "fixture-player-2", "Fixture Player 2", "ふぃくすちゃ二郎", "WR"},
	}
	for _, d := range playerDefs {
		p := &models.Member{Status: models.MSActive}
		p.Slack.ID = d.id
		p.Slack.TeamID = "T9LHPRHA6"
		p.Slack.Name = d.name
		p.Slack.RealName = d.realName
		p.Slack.Profile.RealName = d.realName
		p.Slack.Profile.DisplayName = d.displayName
		p.Slack.Profile.Title = d.title
		entities = append(entities, NewEntity(MemberKey(d.id), p))
	}

	// --- 検証用イベント（#試合 タグ・近未来。ListTapingEvents の絞り込みを通る） ---
	start := now.AddDate(0, 0, 2)
	ev := &models.Event{}
	ev.Google.ID = targetEventID
	ev.Google.Title = "#試合 テーピング検証マッチ"
	ev.Google.StartTime = start.UnixMilli()
	ev.Google.EndTime = start.Add(3 * time.Hour).UnixMilli()
	entities = append(entities, NewEntity(EventKey(targetEventID), ev))

	// --- テープ素材（ID 順 ≠ 名前順） ---
	tapeDefs := []struct {
		id    int64
		name  string
		stock float64
	}{
		{9101, "ホワイトテープ", 20},
		{9102, "アンダーラップ", 10},
		{9103, "キネシオテープ", 5},
	}
	for _, d := range tapeDefs {
		entities = append(entities, NewEntity(TapeItemKey(d.id), &models.TapeItem{
			Name:       d.name,
			StockCount: d.stock,
			SortOrder:  0, // PROD 同様、明示的な表示順は入っていない
		}))
	}

	white := models.TapeUsage{TapeItemID: 9101, TapeItemName: "ホワイトテープ", Quantity: 1}
	under := models.TapeUsage{TapeItemID: 9102, TapeItemName: "アンダーラップ", Quantity: 0.5}

	// --- 施術メニュー（ID 順 ≠ 名前順。かな・漢字混在） ---
	menuDefs := []struct {
		id     int64
		name   string
		price  int
		usages []models.TapeUsage
	}{
		{9001, "膝", 300, []models.TapeUsage{white, under}},
		{9002, "足首", 200, []models.TapeUsage{white}},
		{9003, "肩", 250, []models.TapeUsage{white}},
		{9004, "アキレス腱", 150, []models.TapeUsage{under}},
		{9005, "ふくらはぎ", 180, []models.TapeUsage{white}},
	}
	menuByID := map[int64]*models.TapingMenuItem{}
	for _, d := range menuDefs {
		m := &models.TapingMenuItem{
			Name:       d.name,
			Price:      d.price,
			TapeUsages: d.usages,
			SortOrder:  0, // PROD 同様、明示的な表示順は入っていない
		}
		menuByID[d.id] = m
		entities = append(entities, NewEntity(TapingMenuItemKey(d.id), m))
	}

	// --- 申請済みリクエスト（2名 × 複数項目） ---
	// あえて名前順でない順（キー順とも別）で宣言する。並び順は表示側の責務。
	requestDefs := []struct {
		memberID string
		menuID   int64
	}{
		{playerASlackID, 9001}, // 膝
		{playerASlackID, 9004}, // アキレス腱
		{playerASlackID, 9003}, // 肩
		{playerBSlackID, 9005}, // ふくらはぎ
		{playerBSlackID, 9002}, // 足首
		{playerBSlackID, 9004}, // アキレス腱
	}
	for _, r := range requestDefs {
		m := menuByID[r.menuID]
		entities = append(entities, NewEntity(
			TapingKey(r.memberID, targetEventID, r.menuID),
			&models.Taping{
				MemberID:     r.memberID,
				EventID:      targetEventID,
				MenuItemID:   r.menuID,
				MenuItemName: m.Name,
				Price:        m.Price,
				TapeUsages:   m.TapeUsages,
				RequestedAt:  now.UnixMilli(),
			},
		))
	}

	// 「その他」自由記述の申請も1件だけ置く。#631 の表示（本文が出て金額列が出ない）と
	// #632 の並び（名前順に紛れず常に末尾）を、送信操作を挟まずに検証できるようにする。
	entities = append(entities, NewEntity(
		TapingNoteKey(playerBSlackID, targetEventID),
		&models.Taping{
			MemberID:     playerBSlackID,
			EventID:      targetEventID,
			MenuItemName: "その他",
			Note:         "テープはかぶれにくいものでお願いします",
			RequestedAt:  now.UnixMilli(),
		},
	))

	return Scenario{Name: "taping", Entities: entities}
}

// equipsScenario は備品一覧・回収報告（/equips, /equips/report）の「1件以上ある」
// ケースの回帰確認に使う最小データ（#636 AC-4）。
//
// #636 の本質的な受け入れ条件は「Equip が 0 件でも壊れないこと」であり、この
// scenario は 0 件を回避する対処ではなく、0 件修正が 1 件以上のケースを壊していない
// ことを確認するための検証用 fixture（Issue #636「修正提案 4」で明示的に許容されている）。
// SEED_SCENARIOS のデフォルト（default,taping）には含めない。
//
// default / taping とは entity が重ならない。
func equipsScenario(now time.Time) Scenario {
	entities := []Entity{
		NewEntity(EquipKey(9001), &models.Equip{
			Name:        "フィクスチャ用ヘルメット",
			ForPractice: true,
			ForGame:     true,
			StorageType: models.StorageTypeTakeHome,
		}),
		NewEntity(EquipKey(9002), &models.Equip{
			Name:        "フィクスチャ用タックリングダミー",
			ForPractice: true,
			ForGame:     false,
			StorageType: models.StorageTypeWarehouse,
		}),
	}
	_ = now // 相対日付を持たないため未使用（他 scenario とシグネチャを揃えるための引数）
	return Scenario{Name: "equips", Entities: entities}
}

// positionScenario は「Slack プロフィールのポジション変更が既回答分に反映されない」
// （Issue #536）の回帰を固定するための最小データ。
//
// 本質は「Participation に残った古いポジションのスナップショットを、画面が
// 二度と読まないこと」なので、それを送信操作なしで検証できる状態を作る:
//
//   - Member の現在の Slack Title は "DL"
//   - 同じメンバーの既回答（join）が、7bad57e 以前の形式のまま Datastore に居る。
//     すなわち ParticipationsJSONString に name / picture / title("WR") が残っている。
//     現行の models.Participation はこれらを持たないため decode 時に捨てられるが、
//     フロントには participations_json_str が生のまま渡るので、画面が誤って
//     entry.title を読み戻せば "WR" セクションに出てしまう。
//
// default / taping とは entity が一切重ならないので、単体でも合成でも成立する。
func positionScenario(now time.Time) Scenario {
	const (
		playerSlackID = "UFIXTUREPOSITION1"
		eventID       = "fixture_event_position_01"
		currentTitle  = "DL" // メンバーの現在のポジション（期待される表示先）
		staleTitle    = "WR" // 回答時に保存された古いポジション（表示されてはならない）
	)

	player := &models.Member{Status: models.MSActive}
	player.Slack.ID = playerSlackID
	player.Slack.TeamID = "T9LHPRHA6"
	player.Slack.Name = "fixture-position-player"
	player.Slack.RealName = "Fixture Position Player"
	player.Slack.Profile.RealName = "Fixture Position Player"
	player.Slack.Profile.DisplayName = "ぽじしょん太郎"
	player.Slack.Profile.Title = currentTitle

	start := now.AddDate(0, 0, 4)
	ev := &models.Event{}
	ev.Google.ID = eventID
	ev.Google.Title = "#練習 ポジション反映検証"
	ev.Google.StartTime = start.UnixMilli()
	ev.Google.EndTime = start.Add(3 * time.Hour).UnixMilli()
	// 旧形式（7bad57e 以前）の Participation をそのまま再現する。
	// 現行スキーマに無いキーを含むため、構造体ではなく生の JSON で書く。
	ev.ParticipationsJSONString = fmt.Sprintf(
		`{%q:{"type":"join","params":{},"name":%q,"picture":"","title":%q}}`,
		playerSlackID, player.Slack.Profile.RealName, staleTitle,
	)

	return Scenario{Name: "position", Entities: []Entity{
		NewEntity(MemberKey(playerSlackID), player),
		NewEntity(EventKey(eventID), ev),
	}}
}
