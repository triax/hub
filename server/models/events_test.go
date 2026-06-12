package models

import "testing"

// TestTagSponsor は #sponsor / #スポンサー / ＃sponsor のタイトルで Tag() が "sponsor" を返すことを確認する。
func TestTagSponsor(t *testing.T) {
	cases := []struct {
		title string
	}{
		{"#sponsor 説明会"},
		{"#スポンサー 説明会"},
		{"＃sponsor 説明会"},
		{"2026年 ＃スポンサー 交流イベント"},
	}
	for _, c := range cases {
		ev := Event{}
		ev.Google.Title = c.title
		if got := ev.Tag(); got != ETSponsor {
			t.Errorf("Tag(%q) = %q, want %q", c.title, got, ETSponsor)
		}
	}
}

// TestShouldSkipRemindersSponsor は ETSponsor の ShouldSkipReminders が ETEvent と同じ挙動になることを確認する。
// - RTRSVP は送信する（skip しない）
// - RTEquipment / RTCondition / RTFinalCall は skip する
func TestShouldSkipRemindersSponsor(t *testing.T) {
	sponsor := Event{}
	sponsor.Google.Title = "#sponsor 説明会"

	event := Event{}
	event.Google.Title = "#event 総会"

	reminderTypes := []ReminderType{RTRSVP, RTFinalCall, RTCondition, RTEquipment}
	for _, rt := range reminderTypes {
		gotSponsor := sponsor.ShouldSkipReminders(rt)
		gotEvent := event.ShouldSkipReminders(rt)
		if gotSponsor != gotEvent {
			t.Errorf("ShouldSkipReminders(%q) for sponsor=%v, event=%v; want same result", rt, gotSponsor, gotEvent)
		}
	}
}
