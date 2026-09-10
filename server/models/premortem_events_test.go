package models

import "testing"

// gameEvent / plainEvent は pickUpcomingGame のためだけの最小構築子。
// Datastore には触らないのでエミュレーター無しで回る（#683 AC-19）。
func titledEvent(title string) Event {
	ev := Event{}
	ev.Google.Title = title
	return ev
}

// AC-17 / AC-18 / AC-19: 開始時刻の昇順に並んだ列から、対象試合を 1 件選ぶ規則。
func TestPickUpcomingGame(t *testing.T) {
	nonGames := func(n int) []Event {
		out := make([]Event, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, titledEvent("#練習 通常練習"))
		}
		return out
	}

	cases := []struct {
		name   string
		events []Event
		want   string // 空文字なら「見つからない」
	}{
		{
			name:   "先頭が試合ならそれを採る",
			events: []Event{titledEvent("#試合 vs A"), titledEvent("#試合 vs B")},
			want:   "#試合 vs A",
		},
		{
			// AC-18: FindEventsBetween の Limit(10) を踏襲していたらここで落ちる。
			name:   "非試合が 11 件先行しても最も近い試合を採る",
			events: append(nonGames(11), titledEvent("#試合 vs C")),
			want:   "#試合 vs C",
		},
		{
			// AC-17: #ignore は明示オプトアウト。#試合 が付いていても対象にしない。
			name:   "#ignore 付きの試合は飛ばして次の試合を採る",
			events: []Event{titledEvent("#試合 #ignore 練習試合"), titledEvent("#試合 vs D")},
			want:   "#試合 vs D",
		},
		{
			name:   "試合が無ければ見つからない",
			events: []Event{titledEvent("#練習 通常練習"), titledEvent("#mtg 定例")},
			want:   "",
		},
		{
			name:   "空の列でも落ちない",
			events: nil,
			want:   "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := pickUpcomingGame(c.events)
			if c.want == "" {
				if ok {
					t.Fatalf("pickUpcomingGame() = %q, want not found", got.Google.Title)
				}
				return
			}
			if !ok {
				t.Fatalf("pickUpcomingGame() = not found, want %q", c.want)
			}
			if got.Google.Title != c.want {
				t.Fatalf("pickUpcomingGame() = %q, want %q", got.Google.Title, c.want)
			}
		})
	}
}
