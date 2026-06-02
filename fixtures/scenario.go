// Package fixtures は、設計された seed データ（fixture）をコードとして定義し、
// Datastore エミュレーターへ冪等に投入するための仕組みを提供する。
//
// devdata（本番データのダンプ）とは責務が異なる:
//   - devdata: 本番 Datastore の export。非決定論的・機密含みうる・参照用ツール。
//   - fixtures: mission（tackle/uat）が必要とする最小・決定論的データ。
//     バージョン管理され、機能と共に育つチーム資産。
//
// fixture は通常ビルド対象の Go コードなので、モデルのスキーマ変更があれば
// コンパイラが即座に検出する（//go:build ignore な手動スクリプトが静かに腐った
// 反省を踏まえた設計）。
package fixtures

import (
	"fmt"
	"reflect"

	"cloud.google.com/go/datastore"
)

// Entity は scenario が投入する 1 エンティティ。
//
// Value は models.X へのポインタ（datastore.Put が受け取れる形）。
// Key は stable key（NameKey / IDKey）でなければならない。auto-ID（IncompleteKey）は
// 冪等性を壊すため fixture では使わない。
type Entity struct {
	Key   *datastore.Key
	Value interface{}
	// Override は base（先に合成された scenario）の同一 key を意図的に上書きする宣言。
	// 未宣言のまま同一 key を異なる内容で再定義すると Compose が失敗する。
	Override bool
}

// Scenario は名前付きの fixture 集合。
type Scenario struct {
	Name     string
	Entities []Entity
}

// NewEntity は通常の（上書きしない）Entity を作る。
func NewEntity(key *datastore.Key, value interface{}) Entity {
	return Entity{Key: key, Value: value}
}

// Override は base の同一 key を意図的に上書きする Entity を作る。
// 例: default の Member を、taping 権限を足した姿で taping scenario が使いたい場合。
func Override(key *datastore.Key, value interface{}) Entity {
	return Entity{Key: key, Value: value, Override: true}
}

// keyString は collision 判定・dangling 参照判定に使う安定文字列。
// ancestor も含む完全な key 表現を返す。
func keyString(k *datastore.Key) string {
	if k == nil || k.Incomplete() {
		return ""
	}
	return k.String()
}

// Compose は複数 scenario を宣言順に合成する。
//
// 同一 key の衝突は次のように扱う:
//   - 内容が完全一致 → dedup（重複は許容）
//   - 内容が相違 かつ Override 未宣言 → エラー（LLM が既存 key を別内容で再定義する事故を検出）
//   - 内容が相違 かつ Override 宣言あり → 後勝ちで上書き
func Compose(scenarios ...Scenario) (Scenario, error) {
	out := Scenario{Name: "composed"}
	index := map[string]int{} // keyString -> out.Entities のインデックス

	for _, s := range scenarios {
		for _, e := range s.Entities {
			ks := keyString(e.Key)
			if ks == "" {
				return Scenario{}, fmt.Errorf("scenario %q: entity with nil/incomplete key (value=%T)", s.Name, e.Value)
			}
			i, exists := index[ks]
			if !exists {
				index[ks] = len(out.Entities)
				out.Entities = append(out.Entities, e)
				continue
			}
			// 衝突
			if reflect.DeepEqual(out.Entities[i].Value, e.Value) {
				continue // 完全一致 → dedup
			}
			if !e.Override {
				return Scenario{}, fmt.Errorf(
					"key collision on %s: scenario %q redefines it with different content but no Override() declared",
					ks, s.Name)
			}
			out.Entities[i] = e // 明示 Override → 後勝ち
		}
	}
	return out, nil
}
