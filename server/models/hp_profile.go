package models

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
)

const KindHPProfile = "MemberHPProfile"

// HPCustomField はユーザが自由に追加できる key-value フィールド。
type HPCustomField struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Hidden bool   `json:"hidden"`
}

// MemberHPProfile はメンバーが自己編集するHP向けプロフィール情報。
// Datastore のキーは Member と同じ Slack ID を使って 1:1 対応させる。
type MemberHPProfile struct {
	// 表示名（HP掲載用・イニシャルや偽名も可）
	DisplayName     string `json:"display_name"`
	DisplayNameKana string `json:"display_name_kana"`
	FirstName       string `json:"first_name"`
	FamilyName      string `json:"family_name"`

	// テキスト情報
	Height   int    `json:"height"`
	Weight   int    `json:"weight"`
	Position string `json:"position"`
	Hometown string `json:"hometown"`
	School   string `json:"school"`
	Bio      string `json:"bio"`

	// テキスト情報（HP 掲載項目）
	// 長文になりうる 3 つは Datastore のインデックス付き文字列の 1500 バイト上限を
	// 避けるため noindex にする（検索対象にもしないため実害はない）。
	Role                string `json:"role"`
	Enthusiasm          string `json:"enthusiasm" datastore:",noindex"`
	Watchme             string `json:"watchme" datastore:",noindex"`
	Hobbies             string `json:"hobbies"`
	Favorite            string `json:"favorite"`
	WhatILikeAboutTriax string `json:"what_i_like_about_triax" datastore:",noindex"`

	// ユーザ定義カスタムフィールド
	CustomFields []HPCustomField `json:"custom_fields" datastore:",noindex"`

	// 写真（GCS 上のオブジェクト公開 URL）
	PortraitFormalURL   string   `json:"portrait_formal_url"`
	PortraitCasualURL   string   `json:"portrait_casual_url"`
	AdditionalPhotoURLs []string `json:"additional_photo_urls" datastore:",noindex"`

	// 最終保存時刻。PutHPProfile が保存のたびに設定する（クライアント値は信用しない）。
	// 未保存プロフィールのゼロ値は omitzero でキーごと省略され、外部消費者に
	// ダミー日付（0001-01-01）を見せない。
	UpdatedAt time.Time `json:"updated_at,omitzero"`

	// 掲載制御
	HideFromHP   bool     `json:"hide_from_hp"`
	HiddenFields []string `json:"hidden_fields" datastore:",noindex"`
}

// Positions は公開 API が返すポジションの正規形。
var Positions = []string{"QB", "RB", "WR", "TE", "OL", "DL", "LB", "DB", "K", "P", "Staff", "Coach"}

// positionCanonical は「小文字化した表記 → 正規形」の索引。
var positionCanonical = func() map[string]string {
	m := make(map[string]string, len(Positions))
	for _, p := range Positions {
		m[strings.ToLower(p)] = p
	}
	return m
}()

// positionSeparators は複合表記（"WR/DB" 等）の区切り文字。
const positionSeparators = "/／,、・ 　"

// NormalizePosition は Slack プロフィールの Title 由来の自由表記を Positions の
// 正規形へ寄せる。どの正規形にも寄せられない表記は空文字を返す。
//
//	"staff" → "Staff" / "WR/DB" → "WR" / "Sp" → ""
func NormalizePosition(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if p, ok := positionCanonical[strings.ToLower(s)]; ok {
		return p
	}
	// 複合表記は最初に一致したトークンを採用する。
	// なお、フロント側にも Title を分解する箇所が複数あるが（members.tsx の
	// 正規表現分割、events.$id.tsx の "/" 分割）、区切り文字も候補リストも
	// それぞれ異なる。公開 API が外部に約束する正規形はここを唯一の権威とする。
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(positionSeparators, r)
	})
	for _, tok := range tokens {
		if p, ok := positionCanonical[strings.ToLower(tok)]; ok {
			return p
		}
	}
	return ""
}

// hpFieldZeroers は hidden_fields のキー → 該当フィールドを空にする関数。
// client/models/HPProfile.ts の HIDDEN_FIELD_KEYS と 1:1 で対応させること。
var hpFieldZeroers = map[string]func(*MemberHPProfile){
	"display_name":            func(p *MemberHPProfile) { p.DisplayName = "" },
	"display_name_kana":       func(p *MemberHPProfile) { p.DisplayNameKana = "" },
	"first_name":              func(p *MemberHPProfile) { p.FirstName = "" },
	"family_name":             func(p *MemberHPProfile) { p.FamilyName = "" },
	"height":                  func(p *MemberHPProfile) { p.Height = 0 },
	"weight":                  func(p *MemberHPProfile) { p.Weight = 0 },
	"position":                func(p *MemberHPProfile) { p.Position = "" },
	"hometown":                func(p *MemberHPProfile) { p.Hometown = "" },
	"school":                  func(p *MemberHPProfile) { p.School = "" },
	"bio":                     func(p *MemberHPProfile) { p.Bio = "" },
	"role":                    func(p *MemberHPProfile) { p.Role = "" },
	"enthusiasm":              func(p *MemberHPProfile) { p.Enthusiasm = "" },
	"watchme":                 func(p *MemberHPProfile) { p.Watchme = "" },
	"hobbies":                 func(p *MemberHPProfile) { p.Hobbies = "" },
	"favorite":                func(p *MemberHPProfile) { p.Favorite = "" },
	"what_i_like_about_triax": func(p *MemberHPProfile) { p.WhatILikeAboutTriax = "" },
	"portrait_formal":         func(p *MemberHPProfile) { p.PortraitFormalURL = "" },
	"portrait_casual":         func(p *MemberHPProfile) { p.PortraitCasualURL = "" },
}

// IsEmpty は「掲載して見せる内容が何も無い」ことを表す。
// 判定対象は公開コンテンツのみで、UpdatedAt / HideFromHP / HiddenFields は含めない。
// 公開 API は PublicView() を適用した後のビューに対してこれを評価する。
func (p MemberHPProfile) IsEmpty() bool {
	return p.DisplayName == "" && p.DisplayNameKana == "" &&
		p.FirstName == "" && p.FamilyName == "" &&
		p.Height == 0 && p.Weight == 0 &&
		p.Position == "" && p.Hometown == "" && p.School == "" && p.Bio == "" &&
		p.Role == "" && p.Enthusiasm == "" && p.Watchme == "" &&
		p.Hobbies == "" && p.Favorite == "" && p.WhatILikeAboutTriax == "" &&
		p.PortraitFormalURL == "" && p.PortraitCasualURL == "" &&
		len(p.AdditionalPhotoURLs) == 0 && len(p.CustomFields) == 0
}

// HiddenFieldSet returns HiddenFields as a lookup map.
func (p MemberHPProfile) HiddenFieldSet() map[string]bool {
	set := make(map[string]bool, len(p.HiddenFields))
	for _, f := range p.HiddenFields {
		set[f] = true
	}
	return set
}

// PublicView returns a copy with hidden fields zeroed out.
func (p MemberHPProfile) PublicView() MemberHPProfile {
	if p.HideFromHP {
		return MemberHPProfile{HideFromHP: true}
	}
	out := p
	for f := range p.HiddenFieldSet() {
		if zero, ok := hpFieldZeroers[f]; ok {
			zero(&out)
		}
	}
	// カスタムフィールド: hidden=true のものを除外。
	// レシーバと backing array を共有しないよう新しいスライスに詰め替える
	// （out は p の浅いコピーなので、in-place フィルタは呼び出し元を破壊する）。
	if len(out.CustomFields) > 0 {
		visible := make([]HPCustomField, 0, len(out.CustomFields))
		for _, cf := range out.CustomFields {
			if !cf.Hidden {
				visible = append(visible, cf)
			}
		}
		out.CustomFields = visible
	}
	// 表記ゆれの正規化。保存時にも正規化するが、未再保存の既存データを救済する。
	out.Position = NormalizePosition(out.Position)
	// 掲載ビューでは制御フィールド自体も隠す
	out.HideFromHP = false
	out.HiddenFields = nil
	return out
}

func GetHPProfile(ctx context.Context, slackID string) (*MemberHPProfile, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	profile := &MemberHPProfile{}
	key := datastore.NameKey(KindHPProfile, slackID, nil)
	if err := client.Get(ctx, key, profile); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return profile, nil
		}
		if IsFiledMismatch(err) {
			return profile, nil
		}
		return nil, fmt.Errorf("datastore Get: %w", err)
	}
	return profile, nil
}

// GetMultiHPProfile は単一の Datastore クライアントで全メンバーのプロフィールを一括取得する。
// 戻り値のスライスは members と同じ順序で対応する。取得失敗や未存在のエントリは nil になる。
func GetMultiHPProfile(ctx context.Context, members []Member) ([]*MemberHPProfile, error) {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return nil, fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	keys := make([]*datastore.Key, len(members))
	for i, m := range members {
		keys[i] = datastore.NameKey(KindHPProfile, m.Slack.ID, nil)
	}

	profiles := make([]*MemberHPProfile, len(members))
	for i := range profiles {
		profiles[i] = &MemberHPProfile{}
	}

	errs := client.GetMulti(ctx, keys, profiles)
	if errs != nil {
		if merr, ok := errs.(datastore.MultiError); ok {
			for i, e := range merr {
				if e == datastore.ErrNoSuchEntity || IsFiledMismatch(e) {
					profiles[i] = &MemberHPProfile{}
				} else if e != nil {
					profiles[i] = nil
				}
			}
		} else {
			return nil, fmt.Errorf("datastore GetMulti: %w", errs)
		}
	}
	return profiles, nil
}

func PutHPProfile(ctx context.Context, slackID string, profile *MemberHPProfile) error {
	client, err := datastore.NewClient(ctx, os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		return fmt.Errorf("datastore client: %w", err)
	}
	defer client.Close()

	// 更新時刻はサーバが唯一の権威。UpdateHPProfile / UploadHPPhoto の
	// どちらの保存経路もこの関数を通るため、ここだけで一貫して設定できる。
	profile.UpdatedAt = time.Now().UTC()

	key := datastore.NameKey(KindHPProfile, slackID, nil)
	if _, err := client.Put(ctx, key, profile); err != nil {
		return fmt.Errorf("datastore Put: %w", err)
	}
	return nil
}
