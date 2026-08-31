package models

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestNormalizePosition は Slack Title 由来の自由表記が正規形に寄ることを固定する（Issue #643 AC-4）。
func TestNormalizePosition(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// 完全一致（大文字小文字を無視）
		{"QB", "QB"},
		{"staff", "Staff"},
		{"qb", "QB"},
		{"STAFF", "Staff"},
		{"coach", "Coach"},
		// 前後の空白
		{" WR ", "WR"},
		// 複合表記は最初に一致したトークン
		{"WR/DB", "WR"},
		{"WR／DB", "WR"},
		{"DL・LB", "DL"},
		{"TE、OL", "TE"},
		{"K,P", "K"},
		{"Head Coach", "Coach"},
		// 未知表記
		{"Sp", ""},
		{"", ""},
		{"   ", ""},
		{"マネージャー", ""},
	}
	for _, c := range cases {
		if got := NormalizePosition(c.in); got != c.want {
			t.Errorf("NormalizePosition(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizePosition_ObservedProductionValues は、2026-08-31 時点で本番の公開 API が
// 実際に返していた生の position 値すべてが、Positions ∪ {""} に収まることを固定する
// （Issue #643 AC-5 の pre-merge 判定。本番 URL への実測は deploy 後の /uat で行う）。
func TestNormalizePosition_ObservedProductionValues(t *testing.T) {
	// curl https://hub.triax.football/api/1/public/members | jq '[.members[].hp_profile.position] | unique'
	observed := []string{"", "DB", "DL", "K", "LB", "OL", "P", "QB", "RB", "Staff", "TE", "WR", "staff"}
	// Issue に記載のある、roster 側で観測された表記も併せて固定する。
	observed = append(observed, "Sp", "Coach", "WR/DB")

	for _, raw := range observed {
		got := NormalizePosition(raw)
		if got == "" {
			continue // 空文字は「ポジション不明」として許容される
		}
		if !slices.Contains(Positions, got) {
			t.Errorf("NormalizePosition(%q) = %q, which is not in Positions %v", raw, got, Positions)
		}
	}
}

// TestPublicView_NormalizesPosition は、保存し直していない既存データでも
// 公開ビューでは正規化された position が返ることを固定する（Issue #643 AC-3）。
func TestPublicView_NormalizesPosition(t *testing.T) {
	cases := map[string]string{"staff": "Staff", "WR/DB": "WR", "Sp": "", "QB": "QB"}
	for in, want := range cases {
		p := MemberHPProfile{Position: in}
		if got := p.PublicView().Position; got != want {
			t.Errorf("MemberHPProfile{Position:%q}.PublicView().Position = %q, want %q", in, got, want)
		}
	}
}

// TestPublicView_HidesFields は hidden_fields の各キーが対応するフィールドだけを
// 空にすることを固定する（Issue #643 AC-2 のロジック側）。
func TestPublicView_HidesFields(t *testing.T) {
	full := MemberHPProfile{
		DisplayName: "表示名", DisplayNameKana: "ひょうじめい",
		FirstName: "太郎", FamilyName: "三楽",
		Height: 180, Weight: 80,
		Position: "QB", Hometown: "東京", School: "三楽大", Bio: "ひとこと",
		Role: "パートリーダー", Enthusiasm: "意気込み", Watchme: "注目ポイント",
		Hobbies: "趣味", Favorite: "推し", WhatILikeAboutTriax: "好きなところ",
		PortraitFormalURL: "https://example.com/f.jpg",
		PortraitCasualURL: "https://example.com/c.jpg",
	}

	// hobbies だけを非掲載にしても、他の項目は保持される（AC-2）。
	hidden := full
	hidden.HiddenFields = []string{"hobbies"}
	view := hidden.PublicView()
	if view.Hobbies != "" {
		t.Errorf("hidden hobbies should be empty, got %q", view.Hobbies)
	}
	if view.Role != full.Role || view.Enthusiasm != full.Enthusiasm || view.Bio != full.Bio {
		t.Errorf("non-hidden fields must be preserved, got %+v", view)
	}

	// 全キーを非掲載にすると公開ビューは空になる。
	all := full
	for key := range hpFieldZeroers {
		all.HiddenFields = append(all.HiddenFields, key)
	}
	if got := all.PublicView(); !got.IsEmpty() {
		t.Errorf("PublicView with every field hidden must be empty, got %+v", got)
	}
}

// TestPublicView_DoesNotMutateReceiver は、PublicView が呼び出し元の CustomFields を
// 破壊しないことを固定する（従来の in-place フィルタは backing array を共有していた）。
func TestPublicView_DoesNotMutateReceiver(t *testing.T) {
	p := MemberHPProfile{CustomFields: []HPCustomField{
		{Key: "hidden-one", Value: "v1", Hidden: true},
		{Key: "visible-one", Value: "v2"},
	}}

	view := p.PublicView()

	if len(view.CustomFields) != 1 || view.CustomFields[0].Key != "visible-one" {
		t.Fatalf("public view must contain only visible custom fields, got %+v", view.CustomFields)
	}
	if len(p.CustomFields) != 2 || p.CustomFields[0].Key != "hidden-one" || p.CustomFields[1].Key != "visible-one" {
		t.Errorf("receiver custom fields must not be mutated, got %+v", p.CustomFields)
	}
}

// TestIsEmpty は「見せる内容が何も無い」判定の対象範囲を固定する（Issue #643 AC-6）。
func TestIsEmpty(t *testing.T) {
	if !(MemberHPProfile{}).IsEmpty() {
		t.Error("zero profile must be empty")
	}
	// 制御フィールドと更新時刻は判定に含めない。
	control := MemberHPProfile{
		UpdatedAt:    time.Now(),
		HiddenFields: []string{"bio"},
	}
	if !control.IsEmpty() {
		t.Error("updated_at / hidden_fields must not make a profile non-empty")
	}

	nonEmpty := map[string]MemberHPProfile{
		"display_name":            {DisplayName: "x"},
		"bio":                     {Bio: "x"},
		"height":                  {Height: 170},
		"role":                    {Role: "x"},
		"enthusiasm":              {Enthusiasm: "x"},
		"watchme":                 {Watchme: "x"},
		"hobbies":                 {Hobbies: "x"},
		"favorite":                {Favorite: "x"},
		"what_i_like_about_triax": {WhatILikeAboutTriax: "x"},
		"portrait_formal_url":     {PortraitFormalURL: "https://example.com/f.jpg"},
		"additional_photo_urls":   {AdditionalPhotoURLs: []string{"https://example.com/a.jpg"}},
		"custom_fields":           {CustomFields: []HPCustomField{{Key: "k", Value: "v"}}},
	}
	for name, p := range nonEmpty {
		if p.IsEmpty() {
			t.Errorf("profile with %s set must not be empty", name)
		}
	}
}

// TestUpdatedAtOmitZero は、未保存プロフィールで updated_at キー自体が現れず、
// 保存済みでは RFC3339 で出ることを固定する（Issue #643 AC-7、意思決定 #3）。
func TestUpdatedAtOmitZero(t *testing.T) {
	zero, err := json.Marshal(MemberHPProfile{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(zero), "updated_at") {
		t.Errorf("zero UpdatedAt must be omitted entirely, got %s", zero)
	}

	saved, err := json.Marshal(MemberHPProfile{UpdatedAt: time.Date(2026, 8, 31, 12, 34, 56, 0, time.UTC)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(saved), `"updated_at":"2026-08-31T12:34:56Z"`) {
		t.Errorf("saved UpdatedAt must be RFC3339, got %s", saved)
	}
}

// TestHiddenFieldZeroersCoverPublicTextFields は、hidden_fields のキー集合が
// フロント（client/models/HPProfile.ts の HIDDEN_FIELD_KEYS）と乖離していないことを
// サーバ側から固定する。新しい掲載項目を足したら両方に足す必要がある。
func TestHiddenFieldZeroersCoverPublicTextFields(t *testing.T) {
	want := []string{
		"display_name", "display_name_kana", "first_name", "family_name",
		"height", "weight", "position", "hometown", "school", "bio",
		"role", "enthusiasm", "watchme", "hobbies", "favorite", "what_i_like_about_triax",
		"portrait_formal", "portrait_casual",
	}
	if len(hpFieldZeroers) != len(want) {
		t.Errorf("hpFieldZeroers has %d keys, want %d", len(hpFieldZeroers), len(want))
	}
	for _, key := range want {
		if _, ok := hpFieldZeroers[key]; !ok {
			t.Errorf("hpFieldZeroers is missing key %q", key)
		}
	}
}
