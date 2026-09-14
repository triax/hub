package api

import (
	"errors"
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
