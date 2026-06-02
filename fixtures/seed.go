package fixtures

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"github.com/triax/hub/server/models"
)

// kindOrder は upsert の依存順。参照される側を先に投入する。
var kindOrder = []string{
	models.KindMember,
	models.KindEvent,
	models.KindEquip,
	models.KindNumber,
	models.KindTapeItem,
	models.KindTapingMenuItem,
	models.KindCustody,
	models.KindTaping,
	models.KindApplication,
}

func kindRank(kind string) int {
	for i, k := range kindOrder {
		if k == kind {
			return i
		}
	}
	return len(kindOrder) // 未知 Kind は最後
}

// Load は scenario を Validate してから依存順に Datastore へ upsert する。
//
// key-based upsert（stable key への Put）なので冪等。同じ scenario を複数回 Load しても
// エンティティ数は増えない（既存 key を上書きするだけ）。fixture が管理する key 以外の
// 既存データには触らない。
func Load(ctx context.Context, client *datastore.Client, s Scenario) error {
	if err := Validate(s); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 依存順に安定ソート（同一 Kind 内は scenario 定義順を保つ）。
	ordered := make([]Entity, len(s.Entities))
	copy(ordered, s.Entities)
	stableSortByKindRank(ordered)

	for _, e := range ordered {
		if rule, ok := kindRules[e.Key.Kind]; ok && rule.prepare != nil {
			if err := rule.prepare(e.Value); err != nil {
				return fmt.Errorf("prepare %s: %w", keyString(e.Key), err)
			}
		}
		if _, err := client.Put(ctx, e.Key, e.Value); err != nil {
			return fmt.Errorf("put %s: %w", keyString(e.Key), err)
		}
	}
	return nil
}

// stableSortByKindRank は依存順で安定ソートする（挿入ソート: 件数が小さい fixture 向け）。
func stableSortByKindRank(entities []Entity) {
	for i := 1; i < len(entities); i++ {
		j := i
		for j > 0 && kindRank(entities[j-1].Key.Kind) > kindRank(entities[j].Key.Kind) {
			entities[j-1], entities[j] = entities[j], entities[j-1]
			j--
		}
	}
}
