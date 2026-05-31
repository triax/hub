package models

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/datastore"
)

const KindHPProfile = "MemberHPProfile"

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
	Faculty  string `json:"faculty"`
	Bio      string `json:"bio"`

	// 写真（GCS 上のオブジェクト公開 URL）
	PortraitFormalURL   string   `json:"portrait_formal_url"`
	PortraitCasualURL   string   `json:"portrait_casual_url"`
	AdditionalPhotoURLs []string `json:"additional_photo_urls" datastore:",noindex"`

	// 掲載制御
	HideFromHP   bool     `json:"hide_from_hp"`
	HiddenFields []string `json:"hidden_fields" datastore:",noindex"`
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
	hidden := p.HiddenFieldSet()
	out := p
	if hidden["display_name"] {
		out.DisplayName = ""
	}
	if hidden["display_name_kana"] {
		out.DisplayNameKana = ""
	}
	if hidden["first_name"] {
		out.FirstName = ""
	}
	if hidden["family_name"] {
		out.FamilyName = ""
	}
	if hidden["height"] {
		out.Height = 0
	}
	if hidden["weight"] {
		out.Weight = 0
	}
	if hidden["position"] {
		out.Position = ""
	}
	if hidden["hometown"] {
		out.Hometown = ""
	}
	if hidden["school"] {
		out.School = ""
	}
	if hidden["faculty"] {
		out.Faculty = ""
	}
	if hidden["bio"] {
		out.Bio = ""
	}
	if hidden["portrait_formal"] {
		out.PortraitFormalURL = ""
	}
	if hidden["portrait_casual"] {
		out.PortraitCasualURL = ""
	}
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

	key := datastore.NameKey(KindHPProfile, slackID, nil)
	if _, err := client.Put(ctx, key, profile); err != nil {
		return fmt.Errorf("datastore Put: %w", err)
	}
	return nil
}
