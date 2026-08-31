package fixtures

import (
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/server/models"
)

// stable key ヘルパー。
// 実コード（server/api/*.go）の key 生成戦略に合わせている:
//   - Member       : NameKey（Slack ID）
//   - Event        : NameKey（Google Calendar ID）
//   - Number       : NameKey（背番号文字列）
//   - Taping       : NameKey（memberID_eventID_menuItemID）
//   - Application  : NameKey（申請 ID）
//   - Equip/TapeItem/TapingMenuItem/Custody : IDKey（数値 ID）
//     ※ 実コードは IncompleteKey（auto-ID）を使うが、fixture は冪等性のため
//       明示的な数値 ID を割り当てる。

func MemberKey(slackID string) *datastore.Key {
	return datastore.NameKey(models.KindMember, slackID, nil)
}

func EventKey(googleID string) *datastore.Key {
	return datastore.NameKey(models.KindEvent, googleID, nil)
}

// HPProfileKey は Member と 1:1 対応する HP プロフィールの key（Slack ID）。
func HPProfileKey(slackID string) *datastore.Key {
	return datastore.NameKey(models.KindHPProfile, slackID, nil)
}

func EquipKey(id int64) *datastore.Key {
	return datastore.IDKey(models.KindEquip, id, nil)
}

func NumberKey(number string) *datastore.Key {
	return datastore.NameKey(models.KindNumber, number, nil)
}

func TapeItemKey(id int64) *datastore.Key {
	return datastore.IDKey(models.KindTapeItem, id, nil)
}

func TapingMenuItemKey(id int64) *datastore.Key {
	return datastore.IDKey(models.KindTapingMenuItem, id, nil)
}

// CustodyKey は Equip を ancestor に持つ Custody の key。
func CustodyKey(id int64, equip *datastore.Key) *datastore.Key {
	return datastore.IDKey(models.KindCustody, id, equip)
}

// TapingKey は実コード（taping.go）の NameKey 規約に従う。
func TapingKey(memberID, eventID string, menuItemID int64) *datastore.Key {
	return datastore.NameKey(models.KindTaping, fmt.Sprintf("%s_%s_%d", memberID, eventID, menuItemID), nil)
}

// TapingNoteKey は「その他」自由記述エンティティの key。
// 同じく実コード（taping.go の tapingNoteKeyName）の NameKey 規約に従う。
func TapingNoteKey(memberID, eventID string) *datastore.Key {
	return datastore.NameKey(models.KindTaping, fmt.Sprintf("%s_%s_other", memberID, eventID), nil)
}

func ApplicationKey(id string) *datastore.Key {
	return datastore.NameKey(models.KindApplication, id, nil)
}
