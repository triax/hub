package slackbot

import (
	"log"
	"slices"
	"sort"
	"strings"
)

const (
	// 採る負け筋の上限。few（対象が少ない入力）はさらに絞る。
	premortemMaxRisks    = 3
	premortemMaxRisksFew = 1
	// 1 リスクあたりの「今週やること」の上限。schema では縛れないのでここで切る。
	premortemMaxPrevent = 2
	// Label（目次に出す短い名前）の上限。目次は 1 行に「Label + ポジション + 件数」を
	// 並べるので、Label が長いと折り返して一覧性が消える。
	premortemLabelRuneLimit = 14
)

// rankedRisk は採用された負け筋 1 点。Count は plays から数えた実数で、LLM の自己申告ではない。
type rankedRisk struct {
	premortemRisk
	Count int
}

type riskCount struct {
	Key   string
	Count int
}

// premortemStats は描画に依らない集計結果。件数降順・同数は入力順（決定的）。
// 採用外の候補も含むので、カードのメタ行に出す割合の分母になる。
type premortemStats struct {
	Risks []riskCount
}

// premortemReport は 1 通目・チャートのすべてが読む描画用の入力。
type premortemReport struct {
	Risks []rankedRisk
	Plays []premortemPlay
	Stats premortemStats
}

// rankRisks は LLM が挙げた負け筋候補を plays の risk_keys の実数で並べ替え、上位を採る。
//
// 素直に件数順で採ると、反省スレッドに書かれるのは圧倒的に execution なので 3 点とも
// execution になり premortem にならない。そこで **同じ kind は 1 件まで**という枠の配り方を
// 重ねる（#683）。件数そのものは実データから数え、kind は並べ替えには使わない（#658 の原則）。
//
// 決定的であること（同じ入力 → 同じ出力、同数は risks の入力順）を保証する。
func rankRisks(d premortemDigest, few bool) premortemReport {
	counts := countRiskKeys(d)

	order := make([]int, 0, len(d.Risks))
	for i := range d.Risks {
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return counts[d.Risks[order[i]].Key] > counts[d.Risks[order[j]].Key]
	})

	limit := premortemMaxRisks
	if few {
		limit = premortemMaxRisksFew
	}
	risks := pickRisks(d, order, counts, limit, 2)
	if len(risks) == 0 {
		// 全リスクが 1 件以下。0 件にするより順位のまま見せるほうが役に立つ。
		risks = pickRisks(d, order, counts, limit, 1)
	}
	risks = dedupeOpponents(risks)

	return premortemReport{Risks: risks, Plays: d.Plays, Stats: buildPremortemStats(d, order, counts)}
}

// dedupeOpponents は、採用された負け筋のあいだで同じ相手の前提が繰り返されるのを防ぐ。
// 同じ一文が 2 枚のカードに出ると、読み手には新しい情報が増えたように見えて増えていない。
//
// **採用が確定した後に呼ぶ**（#698 決定 8）。候補の段階で消すと件数の集計と順位付けが狂う。
// 間引きは表示の都合であって、順位の都合ではない。LLM に「重複させるな」と指示しないのは、
// 横断的な調整を頼むと数えさせることになるから（#658 の原則）。
func dedupeOpponents(risks []rankedRisk) []rankedRisk {
	seen := map[string]bool{}
	for i := range risks {
		key := normalizeForQuoteMatch(risks[i].Opponent)
		if key == "" {
			continue
		}
		if seen[key] {
			risks[i].Opponent = ""
			risks[i].OpponentQuote = ""
			continue
		}
		seen[key] = true
	}
	return risks
}

// pickRisks は order の順に minCount 件以上のリスクを limit 件まで採る。
// 1 周目は kind の重複を避け、埋まらなければ 2 周目で重複を許して埋める。
// few（limit=1）のときは 1 周目の 1 件目がそのまま採られるので、kind に関わらず件数最上位になる。
func pickRisks(d premortemDigest, order []int, counts map[string]int, limit, minCount int) []rankedRisk {
	picked := make([]rankedRisk, 0, limit)
	taken := map[int]bool{}
	usedKinds := map[string]bool{}

	eligible := func(i int) bool {
		risk := d.Risks[i]
		return !taken[i] && risk.Key != "" && counts[risk.Key] >= minCount
	}
	take := func(i int) {
		risk := normalizeRisk(d.Risks[i])
		taken[i] = true
		if risk.Kind != "" {
			usedKinds[risk.Kind] = true
		}
		picked = append(picked, rankedRisk{premortemRisk: risk, Count: counts[risk.Key]})
	}

	for _, i := range order { // 1 周目: kind を重ねない
		if len(picked) == limit {
			return picked
		}
		// kind が空（LLM が enum 外を返した等）は「型不明」なので、枠の重複判定に使わない。
		if !eligible(i) || (d.Risks[i].Kind != "" && usedKinds[d.Risks[i].Kind]) {
			continue
		}
		take(i)
	}
	for _, i := range order { // 2 周目: 候補の kind が足りなかったぶんを件数順で埋める
		if len(picked) == limit {
			break
		}
		if !eligible(i) {
			continue
		}
		take(i)
	}
	return picked
}

// normalizeRisk は LLM が挙げた負け筋を描画に耐える形に整える。件数は Structured Outputs
// （strict）では縛れないので、ここが唯一の関門になる。
func normalizeRisk(risk premortemRisk) premortemRisk {
	risk.Kind = normalizeRiskKind(risk.Kind)
	risk.Title = strings.TrimSpace(risk.Title)
	risk.Label = premortemRiskLabel(risk)
	risk.Opponent = strings.TrimSpace(risk.Opponent)
	risk.OpponentQuote = strings.TrimSpace(risk.OpponentQuote)
	risk.Scenario = strings.TrimSpace(risk.Scenario)
	risk.Phase = strings.TrimSpace(risk.Phase)
	risk.Unit = strings.TrimSpace(risk.Unit)
	risk.Signal = strings.TrimSpace(risk.Signal)
	risk.Prevent = normalizePrevent(risk.Prevent)
	risk.Positions = uniqueStrings(trimStrings(risk.Positions))
	risk.Quote = strings.TrimSpace(risk.Quote)
	return risk
}

// normalizeRiskKind は enum 外の値を黙って通さない。schema で縛ってはいるが、
// 平文フォールバック経路やモデル差でずれ得るので、型不明として空にし記録する。
func normalizeRiskKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return ""
	}
	if slices.Contains(premortemKinds, kind) {
		return kind
	}
	log.Printf("[premortem] unknown kind %q; treated as unclassified", kind)
	return ""
}

// premortemRiskLabel は目次と pie の凡例に出す短い名前。strict schema は「必ず埋める」までは
// 縛れないので、空なら Title の切り詰めにフォールバックする。
func premortemRiskLabel(risk premortemRisk) string {
	if label := strings.TrimSpace(risk.Label); label != "" {
		return truncateRunes(label, premortemLabelRuneLimit)
	}
	return truncateRunes(strings.TrimSpace(risk.Title), premortemLabelRuneLimit)
}

func normalizePrevent(actions []string) []string {
	actions = trimStrings(actions)
	if len(actions) > premortemMaxPrevent {
		return actions[:premortemMaxPrevent]
	}
	return actions
}

func countRiskKeys(d premortemDigest) map[string]int {
	counts := map[string]int{}
	for _, play := range d.Plays {
		for _, key := range uniqueStrings(trimStrings(play.RiskKeys)) {
			counts[key]++
		}
	}
	return counts
}

// buildPremortemStats は割合の分母になる集計。採用外の候補も含めて数えるので、
// 「採用した 3 点が全体の何割か」が読める。件数降順・同数は入力順。
func buildPremortemStats(d premortemDigest, order []int, counts map[string]int) premortemStats {
	stats := premortemStats{}
	for _, i := range order {
		key := d.Risks[i].Key
		if key == "" || counts[key] == 0 {
			continue
		}
		stats.Risks = append(stats.Risks, riskCount{Key: key, Count: counts[key]})
	}
	return stats
}

// premortemUnitsSpan は採用リスクが 2 ユニット以上にまたがるかを返す。
// #offence / #defense のようにスコープの定まったチャンネルで打たれるのが基本で、
// 1 ユニットに収まるなら毎行同じ値が並ぶだけなので描かない（#683）。
func premortemUnitsSpan(risks []rankedRisk) bool {
	seen := map[string]struct{}{}
	for _, r := range risks {
		if r.Unit == "" {
			continue
		}
		seen[r.Unit] = struct{}{}
	}
	return len(seen) > 1
}
