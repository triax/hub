package api

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/triax/hub/server/models"
)

func testMember(slackID, realName string) models.Member {
	m := models.Member{Status: models.MSActive}
	m.Slack.ID = slackID
	m.Slack.RealName = realName
	return m
}

// TestBuildPublicEntries は公開 API の除外規則を固定する（Issue #643 AC-6 / AC-9）。
//
// Datastore クライアントがハンドラ内で直接生成されているため、除外・整形だけを
// 純粋関数に切り出してここで検証する（memberCacheControl と同じパターン）。
func TestBuildPublicEntries(t *testing.T) {
	members := []models.Member{
		testMember("U_FILLED", "入力済み 太郎"),
		testMember("U_UNTOUCHED", "未入力 次郎"),
		testMember("U_HIDDEN", "全体非掲載 三郎"),
		testMember("U_ALLFIELDSHIDDEN", "全項目非掲載 四郎"),
		testMember("U_FETCH_FAILED", "取得失敗 五郎"),
		testMember("U_NEWFIELDS_ONLY", "新項目のみ 六郎"),
	}
	profiles := []*models.MemberHPProfile{
		{DisplayName: "入力済み", Bio: "よろしく", Position: "staff"},
		{}, // 未保存（GetMultiHPProfile が ErrNoSuchEntity をゼロ値に置換したもの）
		{DisplayName: "非掲載", Bio: "見えない", HideFromHP: true},
		{DisplayName: "全項目非掲載", Bio: "見えない", HiddenFields: []string{"display_name", "bio"}},
		nil, // 取得に失敗したエントリ
		{Enthusiasm: "今年こそ優勝"},
	}

	entries := buildPublicEntries(members, profiles)

	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.SlackID)
	}
	want := []string{"U_FILLED", "U_NEWFIELDS_ONLY"}
	if len(got) != len(want) {
		t.Fatalf("buildPublicEntries returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildPublicEntries returned %v, want %v", got, want)
		}
	}

	// 公開ビューでは position が正規化される（AC-3）。
	if entries[0].HPProfile.Position != "Staff" {
		t.Errorf("position must be normalized in public entries, got %q", entries[0].HPProfile.Position)
	}
	// 制御フィールドは公開ビューから落ちる。
	if entries[0].HPProfile.HideFromHP || entries[0].HPProfile.HiddenFields != nil {
		t.Errorf("control fields must be stripped, got %+v", entries[0].HPProfile)
	}
	// メンバーの表示名・背番号は Member 側から埋まる。
	if entries[0].Name != "入力済み 太郎" {
		t.Errorf("entry name = %q, want %q", entries[0].Name, "入力済み 太郎")
	}
}

// TestBuildPublicEntries_Empty は、対象が 0 件でも nil ではなく空配列を返すことを固定する
// （JSON で "members": null にせず、外部サイトが素朴に回せるようにするため）。
func TestBuildPublicEntries_Empty(t *testing.T) {
	entries := buildPublicEntries(nil, nil)
	if entries == nil {
		t.Fatal("buildPublicEntries must return an empty slice, not nil")
	}
	if len(entries) != 0 {
		t.Fatalf("buildPublicEntries returned %d entries, want 0", len(entries))
	}
}

// TestBuildPublicEntries_ShorterProfiles は、profiles が members より短い異常時でも
// panic せずに処理できることを固定する（GetMultiHPProfile の契約が崩れた場合の保険）。
func TestBuildPublicEntries_ShorterProfiles(t *testing.T) {
	members := []models.Member{testMember("U_A", "A"), testMember("U_B", "B")}
	profiles := []*models.MemberHPProfile{{DisplayName: "A"}}

	entries := buildPublicEntries(members, profiles)
	if len(entries) != 1 || entries[0].SlackID != "U_A" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

// TestCanEditHPProfile は HP プロフィールの認可規則を固定する（Issue #702 AC-1）。
// 本人は Admin 判定を引かずに可、他人の分は Slack Admin のときだけ可。
func TestCanEditHPProfile(t *testing.T) {
	adminLookup := func(isAdmin bool, err error) (func() (bool, error), *int) {
		calls := 0
		return func() (bool, error) {
			calls++
			return isAdmin, err
		}, &calls
	}

	cases := []struct {
		name        string
		caller      string
		target      string
		isAdmin     bool
		lookupErr   error
		want        bool
		wantErr     bool
		wantLookups int
	}{
		{name: "本人", caller: "U_SELF", target: "U_SELF", isAdmin: false, want: true, wantLookups: 0},
		{name: "Admin が他人", caller: "U_ADMIN", target: "U_OTHER", isAdmin: true, want: true, wantLookups: 1},
		{name: "非 Admin が他人", caller: "U_MEMBER", target: "U_OTHER", isAdmin: false, want: false, wantLookups: 1},
		{name: "Admin 判定の失敗", caller: "U_MEMBER", target: "U_OTHER", lookupErr: errors.New("datastore down"), wantErr: true, wantLookups: 1},
		{name: "セッション不明", caller: "", target: "", isAdmin: false, want: false, wantLookups: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lookup, calls := adminLookup(c.isAdmin, c.lookupErr)
			got, err := canEditHPProfile(c.caller, c.target, lookup)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
			if *calls != c.wantLookups {
				t.Errorf("admin lookup called %d times, want %d", *calls, c.wantLookups)
			}
		})
	}
}

func intPtr(n int) *int { return &n }

// digestFixtureEntries は publicMembersDigest のテスト用 entries を毎回新しく作る
// （ケースごとに 1 箇所だけ書き換えても、他のケースに波及しないようにするため）。
func digestFixtureEntries() []publicEntry {
	return []publicEntry{
		{SlackID: "U_A", Name: "A 太郎", Number: intPtr(1), HPProfile: models.MemberHPProfile{
			DisplayName: "A", Bio: "よろしく",
			CustomFields: []models.HPCustomField{{Key: "好きな技", Value: "ラン"}, {Key: "座右の銘", Value: "一歩"}},
		}},
		{SlackID: "U_B", Name: "B 次郎", Number: nil, HPProfile: models.MemberHPProfile{DisplayName: "B"}},
		{SlackID: "U_C", Name: "C 三郎", Number: intPtr(99), HPProfile: models.MemberHPProfile{DisplayName: "C", Position: "QB"}},
	}
}

// TestPublicMembersDigest_OrderIndependent は並び順だけ違う entries が同じ digest になることを固定する（Issue #704 AC-1）。
func TestPublicMembersDigest_OrderIndependent(t *testing.T) {
	entries := digestFixtureEntries()
	want := publicMembersDigest(entries)

	reversed := digestFixtureEntries()
	slices.Reverse(reversed)
	if got := publicMembersDigest(reversed); got != want {
		t.Errorf("digest of reordered entries = %s, want %s", got, want)
	}
	// 呼び出し元の並び順を壊さない（コピーをソートする）。
	if reversed[0].SlackID != "U_C" {
		t.Errorf("publicMembersDigest must not reorder its input, got first = %s", reversed[0].SlackID)
	}
}

// TestPublicMembersDigest_DetectsChange は 1 名の 1 フィールドの変化や 1 名の除外で digest が変わることを固定する（Issue #704 AC-2 / AC-3）。
// custom_fields の並べ替えも公開内容の変化とみなす（homepage#35 との約束）。
func TestPublicMembersDigest_DetectsChange(t *testing.T) {
	base := publicMembersDigest(digestFixtureEntries())

	cases := []struct {
		name   string
		mutate func([]publicEntry) []publicEntry
	}{
		{name: "AC-2 Number を変える", mutate: func(e []publicEntry) []publicEntry { e[0].Number = intPtr(2); return e }},
		{name: "AC-2 Number を nil から付与", mutate: func(e []publicEntry) []publicEntry { e[1].Number = intPtr(7); return e }},
		{name: "AC-2 Number を剥奪", mutate: func(e []publicEntry) []publicEntry { e[2].Number = nil; return e }},
		{name: "AC-2 Name を変える", mutate: func(e []publicEntry) []publicEntry { e[1].Name = "B 二郎"; return e }},
		{name: "AC-2 HPProfile の 1 フィールドを変える", mutate: func(e []publicEntry) []publicEntry { e[2].HPProfile.Bio = "新しい自己紹介"; return e }},
		{name: "AC-2 写真 URL の差し替え", mutate: func(e []publicEntry) []publicEntry {
			e[0].HPProfile.PortraitFormalURL = "https://example.com/hp/photos/U_A/formal-2.jpg"
			return e
		}},
		{name: "AC-3 1 名を取り除く", mutate: func(e []publicEntry) []publicEntry { return e[1:] }},
		{name: "custom_fields の並べ替え", mutate: func(e []publicEntry) []publicEntry {
			slices.Reverse(e[0].HPProfile.CustomFields)
			return e
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := publicMembersDigest(c.mutate(digestFixtureEntries())); got == base {
				t.Errorf("digest did not change: %s", got)
			}
		})
	}
}

// TestPublicMembersDigest_Empty は 0 件でも panic せず、nil / 空スライスとも同じ固定値になることを固定する（Issue #704 AC-4）。
func TestPublicMembersDigest_Empty(t *testing.T) {
	// sha256("[]")
	const want = "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"
	if got := publicMembersDigest([]publicEntry{}); got != want {
		t.Errorf("digest of empty entries = %s, want %s", got, want)
	}
	if got := publicMembersDigest(nil); got != want {
		t.Errorf("digest of nil entries = %s, want %s", got, want)
	}
}

// TestPublicMembersDigest_Stable は同じ内容なら何度計算しても同じ値になることを固定する（Issue #704 AC-6 の純粋関数側）。
func TestPublicMembersDigest_Stable(t *testing.T) {
	first := publicMembersDigest(digestFixtureEntries())
	if !strings.HasPrefix(first, "sha256:") || len(first) != len("sha256:")+64 {
		t.Fatalf("unexpected digest format: %s", first)
	}
	if second := publicMembersDigest(digestFixtureEntries()); second != first {
		t.Errorf("digest is not stable: %s != %s", second, first)
	}
}
