package models

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
)

// FindEventsBetween の件数上限を外した挙動（#687）は、実クエリでしか捕まえられない。
// 純粋関数に切り出せる部分が無いので、Datastore エミュレーターに対する統合テストで縛る。
//
// CI（.github/workflows/go.yml の `go test -v ./...`）にはエミュレーターが無いので、
// DATASTORE_EMULATOR_HOST が設定されているときだけ走らせる。ローカルでは
// `scripts/env-provision provision <id>` が立てたエミュレーターを使う:
//
//	DATASTORE_EMULATOR_HOST=localhost:9298 GOOGLE_CLOUD_PROJECT=triax-hub-local \
//	  go test ./server/models/ -run TestFindEventsBetween -v
func requireEmulator(t *testing.T) {
	t.Helper()
	if os.Getenv("DATASTORE_EMULATOR_HOST") == "" {
		t.Skip("DATASTORE_EMULATOR_HOST 未設定のため skip（エミュレーターが要る統合テスト）")
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		t.Setenv("GOOGLE_CLOUD_PROJECT", "triax-hub-test")
	}
}

// emulatorClient は seed / cleanup のためのクライアント。読むだけのテストは
// requireEmulator だけ呼べばよい（FindEventsBetween が自前でクライアントを作るため）。
func emulatorClient(t *testing.T) *datastore.Client {
	t.Helper()
	requireEmulator(t)
	client, err := datastore.NewClient(context.Background(), os.Getenv("GOOGLE_CLOUD_PROJECT"))
	if err != nil {
		t.Fatalf("datastore.NewClient: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// seedEvents は互いに 1 時間ずらした n 件のイベントを投入し、後片付けまで面倒を見る。
// 既存の fixture と混ざらないよう、遠い未来（2099 年）に置く。
func seedEvents(t *testing.T, client *datastore.Client, prefix string, base time.Time, n int) {
	t.Helper()
	ctx := context.Background()
	keys := make([]*datastore.Key, 0, n)
	entities := make([]*Event, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s_%02d", prefix, i)
		ev := &Event{}
		ev.Google.ID = id
		ev.Google.Title = fmt.Sprintf("#練習 %s", id)
		ev.Google.StartTime = base.Add(time.Duration(i)*time.Hour).Unix() * 1000
		ev.Google.EndTime = ev.Google.StartTime + 2*60*60*1000
		keys = append(keys, datastore.NameKey(KindEvent, id, nil))
		entities = append(entities, ev)
	}
	if _, err := client.PutMulti(ctx, keys, entities); err != nil {
		t.Fatalf("PutMulti: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteMulti(context.Background(), keys); err != nil {
			t.Logf("cleanup DeleteMulti: %v", err)
		}
	})
}

// waitForEvents は want 件が見えるまで FindEventsBetween を叩き直す。
//
// エミュレーターは --consistency 未指定だと既定 0.9 で、インデックス読みの約 1 割が stale に
// なる（scripts/env-provision は指定していない）。PutMulti 直後の 1 回きりの問い合わせだと
// 14/15 のように取りこぼして落ちるため、締め切りまで待つ。
//
// これで検出力が落ちることはない。**Limit(10) が残っていれば何秒待っても 10 件を超えない**ので、
// タイムアウトして落ちる。
func waitForEvents(t *testing.T, from, to time.Time, want int) []Event {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var events []Event
	for {
		var err error
		events, err = FindEventsBetween(context.Background(), from, to)
		if err != nil {
			t.Fatalf("FindEventsBetween: %v", err)
		}
		if len(events) >= want || time.Now().After(deadline) {
			return events
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// #687 AC-2: 窓に 11 件以上あっても全件返る（以前は Limit(10) で「最も遠い 10 件」に切られていた）。
func TestFindEventsBetween_NoLimit(t *testing.T) {
	client := emulatorClient(t)

	const n = 15
	base := time.Date(2099, 3, 1, 9, 0, 0, 0, time.UTC)
	seedEvents(t, client, "issue687_nolimit", base, n)

	events := waitForEvents(t, base.Add(-time.Hour), base.Add(time.Duration(n)*time.Hour), n)
	if len(events) != n {
		t.Fatalf("件数 = %d, want %d（Limit で切られていないこと）", len(events), n)
	}

	// 並びは降順のまま。先頭が「窓の中で最も新しい」＝ events[0] を使う既存呼び出し元の前提。
	for i := 1; i < len(events); i++ {
		if events[i-1].Google.StartTime < events[i].Google.StartTime {
			t.Fatalf("降順で並んでいない: [%d]=%d < [%d]=%d",
				i-1, events[i-1].Google.StartTime, i, events[i].Google.StartTime)
		}
	}
	if want := base.Add(time.Duration(n-1)*time.Hour).Unix() * 1000; events[0].Google.StartTime != want {
		t.Fatalf("events[0] が最新でない: %d, want %d", events[0].Google.StartTime, want)
	}

	// 上限で切られていたら「最も近い方」が落ちる。それが起きていないことを明示的に見る。
	if last := events[len(events)-1].Google.StartTime; last != base.Unix()*1000 {
		t.Fatalf("最も古い 1 件が落ちている: %d, want %d", last, base.Unix()*1000)
	}
}

// #687 AC-6: 窓にイベントが 1 件も無ければ空を返す（エラーにしない）。
// EquipsScanUnreported が「イベント無し」として正常応答できる前提。
func TestFindEventsBetween_EmptyWindow(t *testing.T) {
	requireEmulator(t)

	base := time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC)
	events, err := FindEventsBetween(context.Background(), base, base.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("FindEventsBetween: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("件数 = %d, want 0", len(events))
	}
}

// #687 AC-5: EquipsScanUnreported の「直近の過去イベント」が、下限を入れる前後で変わらない。
//
// あの endpoint は from にゼロ値（＝過去全件）を渡して events[0] を取っていた。Limit を
// 外すのに合わせて 1 年の下限を入れたので、「下限あり」と「下限なし」で同じイベントを
// 引くことを実測で押さえる。
func TestFindEventsBetween_LookbackKeepsLatest(t *testing.T) {
	client := emulatorClient(t)

	// 2 年前・30 日前・1 日前（いずれも過去）。期待する latest は 1 日前。
	now := time.Now().UTC().Truncate(time.Hour)
	offsets := []time.Duration{-2 * 365 * 24 * time.Hour, -30 * 24 * time.Hour, -24 * time.Hour}
	keys := make([]*datastore.Key, 0, len(offsets))
	entities := make([]*Event, 0, len(offsets))
	for i, d := range offsets {
		id := fmt.Sprintf("issue687_lookback_%02d", i)
		ev := &Event{}
		ev.Google.ID = id
		ev.Google.Title = "#練習 " + id
		ev.Google.StartTime = now.Add(d).Unix() * 1000
		keys = append(keys, datastore.NameKey(KindEvent, id, nil))
		entities = append(entities, ev)
	}
	if _, err := client.PutMulti(context.Background(), keys, entities); err != nil {
		t.Fatalf("PutMulti: %v", err)
	}
	t.Cleanup(func() { _ = client.DeleteMulti(context.Background(), keys) })

	wantID := "issue687_lookback_02"
	latestOf := func(name string, from time.Time) string {
		deadline := time.Now().Add(5 * time.Second)
		for {
			events, err := FindEventsBetween(context.Background(), from, now)
			if err != nil {
				t.Fatalf("%s: FindEventsBetween: %v", name, err)
			}
			if len(events) > 0 && events[0].Google.ID == wantID {
				return events[0].Google.ID
			}
			if time.Now().After(deadline) {
				if len(events) == 0 {
					t.Fatalf("%s: イベントが 1 件も返らない", name)
				}
				return events[0].Google.ID
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 下限なし（変更前の呼び方）と 1 年の下限（変更後）で同じイベントを引く。
	unbounded := latestOf("下限なし", time.Time{})
	bounded := latestOf("1 年の下限", now.Add(-365*24*time.Hour))
	if unbounded != bounded {
		t.Fatalf("latest が変わった: 下限なし=%q / 1 年の下限=%q", unbounded, bounded)
	}
	if bounded != wantID {
		t.Fatalf("latest = %q, want %q（最も新しい過去イベント）", bounded, wantID)
	}
}
