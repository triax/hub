package slackbot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// 入力が focusPromptRuneBudget を超えると splitThreadsForPrompt が塊に分け、塊ごとに
// LLM を呼ぶ。塊どうしは互いを見ていないので、同じ課題が別 key で重複して挙がり、
// 件数が分散して順位が壊れる（focus は 1〜3 点に収束せず、premortem は kind の
// 重複回避が塊をまたいで効かない）。
//
// そこで 2 段目として「候補の名寄せ」だけを LLM にさせる（#688）。原則は 2 つ:
//
//   - **判断だけさせて記述はさせない**。返させるのは key のグルーピングのみで、
//     title / summary / quote といった本文は 1 段目の出力をそのまま使う。本文を
//     書き直させると、根拠に紐づいていた記述が 2 段目の幻覚に置き換わる。
//   - **件数は数えさせない**（#658）。plays は 2 段目に渡さず、名寄せ後に Hub 側で数え直す。

const mergeSchemaName = "candidate_merge"

// mergeCandidate は 2 段目に渡す候補の最小表現。focusTheme からも premortemRisk からも
// 作れる形にして、名寄せ本体を digest の型に依存させない。
type mergeCandidate struct {
	Key   string
	Title string
	Label string
}

// mergeGroup は「同じことを言っている候補」の 1 グループ。
type mergeGroup struct {
	CanonicalKey string   `json:"canonical_key"`
	MemberKeys   []string `json:"member_keys"`
}

type mergeGroups struct {
	Groups []mergeGroup `json:"groups"`
}

// mergeSchema は本文フィールドを一切持たない。2 段目に記述をさせないことを schema で縛る。
var mergeSchema = strictObject(map[string]any{
	"groups": arrayOf(strictObject(map[string]any{
		"canonical_key": stringField(),
		"member_keys":   arrayOf(stringField()),
	})),
})

func mergeSystemPrompt() string {
	return `あなたはアメリカンフットボールチームのコーチ補佐です。
入力は、同じ期間の反省を分割して読んだために**重複して挙がった課題候補の一覧**です。
1 行が 1 候補で ` + "`key | 見出し | 短い名前`" + ` の形をしています。

# やること
- 同じことを言っている候補をグループにまとめる。
- canonical_key には、そのグループの中で最も的確な見出しを持つ候補の key を選ぶ。
- member_keys には、そのグループに属する全ての key を入れる（canonical_key 自身も含める）。
- どのグループにも属さない候補は、**その候補 1 件だけのグループ**にする。
  結果として、入力の全 key がちょうど 1 回ずつどこかの member_keys に現れる。

# まとめる / まとめない の基準
- 表記や語彙が違っても、指している現象が同じならまとめる
  （例: 「ブレイク前の減速」と「カット前に緩む」は同じ）。
- 症状が同じでも原因が違うならまとめない
  （例: 「パスが通らない（QB の判断）」と「パスが通らない（WR のルート）」は別）。
- 迷ったらまとめない。無関係なものを 1 つにすると、元の指摘に戻れなくなる。

# 禁止
- key を新しく作らない。入力に現れた key だけを使う。
- 見出しや説明文を書かない。返すのは key のグループ分けだけ。`
}

// renderMergeCandidates は候補一覧を 2 段目のプロンプト本文に整形する。
// plays は渡さない（件数を数えさせないため）。
func renderMergeCandidates(candidates []mergeCandidate) string {
	buf := &strings.Builder{}
	for _, c := range candidates {
		fmt.Fprintf(buf, "%s | %s | %s\n", c.Key, c.Title, c.Label)
	}
	return buf.String()
}

// mergeCandidateKeys は 2 段目を 1 回だけ呼び、「寄せられる key → 代表 key」の対応表を返す。
// 名寄せが要らない・できない場合は空の map を返す。**2 段目の失敗で全体を落とさない**のが
// この関数の契約で、呼び出し側は空 map を「名寄せしない」として扱えばよい。
func (bot Bot) mergeCandidateKeys(ctx context.Context, candidates []mergeCandidate) map[string]string {
	if len(candidates) < 2 {
		return nil // 寄せる相手がいない
	}
	reply, err := bot.chat(ctx, ChatRequest{
		Model:  chatModelFocus, // 名寄せは意味判断。ここを軽いモデルにすると 1 段目の質ごと台無しになる
		System: []string{mergeSystemPrompt()},
		User:   renderMergeCandidates(candidates),
		Schema: &ChatJSONSchema{Name: mergeSchemaName, Schema: mergeSchema},
	})
	if err != nil {
		log.Printf("[merge] 2 段目の呼び出しに失敗、名寄せせず続行します: %v", err)
		return nil
	}
	groups := mergeGroups{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(reply)), &groups); err != nil {
		log.Printf("[merge] 2 段目の応答を parse できず、名寄せせず続行します: %v", err)
		return nil
	}
	return resolveMergeMap(groups, candidates)
}

// resolveMergeMap は応答を「実際に寄せる分だけ」の対応表に落とす。
// LLM の応答は信用しない: 入力に無い key、代表が未知のグループ、同じ key の重複出現は捨てる。
// 恒等な対応（代表が自分自身）は入れないので、返る map が空なら「名寄せ無し」を意味する。
func resolveMergeMap(groups mergeGroups, candidates []mergeCandidate) map[string]string {
	known := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		known[c.Key] = struct{}{}
	}

	mapping := map[string]string{}
	assigned := map[string]struct{}{} // 同じ key が複数グループに出たら先勝ち
	for _, g := range groups.Groups {
		canonical := strings.TrimSpace(g.CanonicalKey)
		if _, ok := known[canonical]; !ok {
			continue // 代表が入力に無い（key を新造された）グループごと捨てる
		}
		if _, ok := assigned[canonical]; ok {
			continue
		}
		assigned[canonical] = struct{}{}
		for _, member := range g.MemberKeys {
			member = strings.TrimSpace(member)
			if member == "" || member == canonical {
				continue
			}
			if _, ok := known[member]; !ok {
				continue
			}
			if _, ok := assigned[member]; ok {
				continue
			}
			assigned[member] = struct{}{}
			mapping[member] = canonical
		}
	}
	return mapping
}

// toMergeCandidates は digest の候補一覧から 2 段目に渡す最小表現を作る。
// key が空のものは名寄せのしようがないので落とす。digest の型に依存させないため、
// 1 件ぶんの取り出しだけ呼び出し側から受け取る。
func toMergeCandidates[T any](items []T, extract func(T) mergeCandidate) []mergeCandidate {
	out := make([]mergeCandidate, 0, len(items))
	for _, item := range items {
		if c := extract(item); c.Key != "" {
			out = append(out, c)
		}
	}
	return out
}

// canonicalKeys は key 列を代表 key に張り替え、重複を落とす（入力順は保つ）。
// 対応表に無い key はそのまま通る。
func canonicalKeys(keys []string, mapping map[string]string) []string {
	out := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, key := range keys {
		if canonical, ok := mapping[key]; ok {
			key = canonical
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// keepCanonical は他へ寄せられた候補を落とし、代表だけを入力順で残す。
// digest の型に依存しないよう、key の取り出しだけ呼び出し側から受け取る。
func keepCanonical[T any](items []T, keyOf func(T) string, mapping map[string]string) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if _, merged := mapping[keyOf(item)]; merged {
			continue
		}
		out = append(out, item)
	}
	return out
}
