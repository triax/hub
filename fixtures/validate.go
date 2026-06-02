package fixtures

import (
	"encoding/json"
	"fmt"
	"reflect"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/server/models"
)

// kindRule は Kind ごとのセマンティック検証・参照抽出・投入前準備をまとめる。
// すべて任意（nil なら該当チェックをスキップ）。
type kindRule struct {
	// required は必須フィールドの非空を検証する。
	required func(v interface{}) error
	// refs はこの entity が参照する他 entity の key を返す（dangling 検出用）。
	refs func(v interface{}) []*datastore.Key
	// roundtrip は JSON エンコードフィールドの marshal/unmarshal 健全性を検証する。
	roundtrip func(v interface{}) error
	// prepare は Put 直前に JSON エンコードフィールド等を埋める（投入用）。
	prepare func(v interface{}) error
}

func typeErr(kind string, v interface{}) error {
	return fmt.Errorf("kind %s: unexpected value type %T", kind, v)
}

var kindRules = map[string]kindRule{
	models.KindMember: {
		required: func(v interface{}) error {
			m, ok := v.(*models.Member)
			if !ok {
				return typeErr(models.KindMember, v)
			}
			if m.Slack.ID == "" {
				return fmt.Errorf("Member.Slack.ID is empty")
			}
			return nil
		},
	},
	models.KindEvent: {
		required: func(v interface{}) error {
			e, ok := v.(*models.Event)
			if !ok {
				return typeErr(models.KindEvent, v)
			}
			if e.Google.ID == "" {
				return fmt.Errorf("Event.Google.ID is empty")
			}
			if e.Google.Title == "" {
				return fmt.Errorf("Event.Google.Title is empty")
			}
			return nil
		},
		roundtrip: func(v interface{}) error {
			e := v.(*models.Event)
			if e.ParticipationsJSONString == "" {
				return nil
			}
			if _, err := e.Participations(); err != nil {
				return fmt.Errorf("Event.ParticipationsJSONString invalid: %w", err)
			}
			return nil
		},
	},
	models.KindEquip: {
		required: func(v interface{}) error {
			e, ok := v.(*models.Equip)
			if !ok {
				return typeErr(models.KindEquip, v)
			}
			if e.Name == "" {
				return fmt.Errorf("Equip.Name is empty")
			}
			return nil
		},
	},
	models.KindNumber: {
		required: func(v interface{}) error {
			if _, ok := v.(*models.PlayerNumber); !ok {
				return typeErr(models.KindNumber, v)
			}
			return nil
		},
		refs: func(v interface{}) []*datastore.Key {
			n := v.(*models.PlayerNumber)
			if n.PlayerID == "" {
				return nil
			}
			return []*datastore.Key{MemberKey(n.PlayerID)}
		},
	},
	models.KindTapeItem: {
		required: func(v interface{}) error {
			t, ok := v.(*models.TapeItem)
			if !ok {
				return typeErr(models.KindTapeItem, v)
			}
			if t.Name == "" {
				return fmt.Errorf("TapeItem.Name is empty")
			}
			return nil
		},
	},
	models.KindTapingMenuItem: {
		required: func(v interface{}) error {
			mi, ok := v.(*models.TapingMenuItem)
			if !ok {
				return typeErr(models.KindTapingMenuItem, v)
			}
			if mi.Name == "" {
				return fmt.Errorf("TapingMenuItem.Name is empty")
			}
			return nil
		},
		roundtrip: func(v interface{}) error {
			mi := v.(*models.TapingMenuItem)
			return marshalTapeUsages(mi.TapeUsages, mi.TapeUsagesJSON)
		},
		prepare: func(v interface{}) error {
			mi := v.(*models.TapingMenuItem)
			b, err := json.Marshal(mi.TapeUsages)
			if err != nil {
				return fmt.Errorf("TapingMenuItem.TapeUsages marshal: %w", err)
			}
			mi.TapeUsagesJSON = string(b)
			return nil
		},
	},
	models.KindCustody: {
		required: func(v interface{}) error {
			if _, ok := v.(*models.Custody); !ok {
				return typeErr(models.KindCustody, v)
			}
			return nil
		},
		refs: func(v interface{}) []*datastore.Key {
			c := v.(*models.Custody)
			if c.MemberID == "" {
				return nil
			}
			return []*datastore.Key{MemberKey(c.MemberID)}
		},
	},
	models.KindTaping: {
		required: func(v interface{}) error {
			t, ok := v.(*models.Taping)
			if !ok {
				return typeErr(models.KindTaping, v)
			}
			if t.MemberID == "" || t.EventID == "" {
				return fmt.Errorf("Taping.MemberID / Taping.EventID must be set")
			}
			return nil
		},
		refs: func(v interface{}) []*datastore.Key {
			t := v.(*models.Taping)
			return []*datastore.Key{MemberKey(t.MemberID), EventKey(t.EventID)}
		},
		roundtrip: func(v interface{}) error {
			t := v.(*models.Taping)
			return marshalTapeUsages(t.TapeUsages, t.TapeUsagesJSON)
		},
		prepare: func(v interface{}) error {
			t := v.(*models.Taping)
			b, err := json.Marshal(t.TapeUsages)
			if err != nil {
				return fmt.Errorf("Taping.TapeUsages marshal: %w", err)
			}
			t.TapeUsagesJSON = string(b)
			return nil
		},
	},
	models.KindApplication: {
		required: func(v interface{}) error {
			a, ok := v.(*models.Application)
			if !ok {
				return typeErr(models.KindApplication, v)
			}
			if a.Email == "" {
				return fmt.Errorf("Application.Email is empty")
			}
			return nil
		},
		roundtrip: func(v interface{}) error {
			a := v.(*models.Application)
			if a.FieldsJSON == "" {
				return nil
			}
			tmp := map[string]string{}
			if err := json.Unmarshal([]byte(a.FieldsJSON), &tmp); err != nil {
				return fmt.Errorf("Application.FieldsJSON invalid: %w", err)
			}
			return nil
		},
		prepare: func(v interface{}) error {
			a := v.(*models.Application)
			return a.MarshalFields()
		},
	},
}

// marshalTapeUsages は TapeUsages（struct）が JSON 化可能か検証する。
// 既存 JSON 文字列がある場合はそれもパース可能かを確認する。
func marshalTapeUsages(usages []models.TapeUsage, existingJSON string) error {
	if _, err := json.Marshal(usages); err != nil {
		return fmt.Errorf("TapeUsages marshal: %w", err)
	}
	if existingJSON != "" {
		var tmp []models.TapeUsage
		if err := json.Unmarshal([]byte(existingJSON), &tmp); err != nil {
			return fmt.Errorf("TapeUsagesJSON invalid: %w", err)
		}
	}
	return nil
}

// Validate は scenario のセマンティック健全性を検証する。
// コンパイラは型 drift しか守らないため、以下を実行時に検査する:
//
//	(a) すべての key が非空（stable key であること）
//	(b) 必須フィールドが非空（Kind ごと）
//	(c) JSON エンコードフィールドの marshal/unmarshal roundtrip
//	(d) 参照先 entity の存在（dangling key 検出）
//	(e) 同一 key の内容差分 collision（Compose 後の最終確認）
func Validate(s Scenario) error {
	present := map[string]bool{}
	seen := map[string]interface{}{}

	for _, e := range s.Entities {
		// (a) key 非空
		ks := keyString(e.Key)
		if ks == "" {
			return fmt.Errorf("entity with nil/incomplete key (value=%T)", e.Value)
		}
		// (e) collision（Compose を経ずに直接 Validate された場合の保険）
		if prev, ok := seen[ks]; ok {
			if !equalValue(prev, e.Value) {
				return fmt.Errorf("duplicate key %s with different content", ks)
			}
		}
		seen[ks] = e.Value
		present[ks] = true

		rule, ok := kindRules[e.Key.Kind]
		if !ok {
			return fmt.Errorf("no validation rule registered for kind %q", e.Key.Kind)
		}
		// (b) 必須フィールド
		if rule.required != nil {
			if err := rule.required(e.Value); err != nil {
				return fmt.Errorf("%s: %w", ks, err)
			}
		}
		// (c) JSON roundtrip
		if rule.roundtrip != nil {
			if err := rule.roundtrip(e.Value); err != nil {
				return fmt.Errorf("%s: %w", ks, err)
			}
		}
	}

	// (d) dangling 参照
	for _, e := range s.Entities {
		rule := kindRules[e.Key.Kind]
		if rule.refs == nil {
			continue
		}
		for _, ref := range rule.refs(e.Value) {
			rks := keyString(ref)
			if rks == "" {
				return fmt.Errorf("%s references an incomplete key", keyString(e.Key))
			}
			if !present[rks] {
				return fmt.Errorf("%s references %s which is not present in the scenario (dangling key)",
					keyString(e.Key), rks)
			}
		}
	}
	return nil
}

func equalValue(a, b interface{}) bool {
	// Compose と同じ判定（reflect.DeepEqual）に委ねる。
	return reflect.DeepEqual(a, b)
}
