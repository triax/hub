package models

import "testing"

// TestTagsSingle は単一タグのタイトルで Tags() が該当タグを含むことを確認する。
func TestTagsSingle(t *testing.T) {
	cases := []struct {
		title string
		want  EventTag
	}{
		{"#sponsor 説明会", ETSponsor},
		{"#スポンサー 説明会", ETSponsor},
		{"＃sponsor 説明会", ETSponsor},
		{"2026年 ＃スポンサー 交流イベント", ETSponsor},
		{"#練習 春季", ETPractice},
		{"#試合 vs X", ETGame},
		{"#event 総会", ETEvent},
		{"#meeting 定例", ETMeeting},
		{"#mtg 定例", ETMeeting},
		{"#ignore テスト", ETIgnore},
		{"タグなしイベント", ETUnkonwn},
	}
	for _, c := range cases {
		ev := Event{}
		ev.Google.Title = c.title
		if !ev.HasTag(c.want) {
			t.Errorf("Tags(%q) = %v, want to contain %q", c.title, ev.Tags(), c.want)
		}
	}
}

// TestTagsMultiple は複数タグ併記のタイトルで Tags() が全タグを返すことを確認する。
func TestTagsMultiple(t *testing.T) {
	ev := Event{}
	ev.Google.Title = "#練習 #sponsor 合同練習＆協賛社見学"
	if !ev.HasTag(ETPractice) || !ev.HasTag(ETSponsor) {
		t.Errorf("Tags(%q) = %v, want to contain both %q and %q", ev.Google.Title, ev.Tags(), ETPractice, ETSponsor)
	}
}

// TestTagsMtgFalsePositive は # の無い "mtg" 部分文字列が meeting に誤判定されないことを確認する。
func TestTagsMtgFalsePositive(t *testing.T) {
	ev := Event{}
	ev.Google.Title = "important mtg notes" // # が無い
	if ev.HasTag(ETMeeting) {
		t.Errorf("Tags(%q) = %v, must not contain %q (# が無いため)", ev.Google.Title, ev.Tags(), ETMeeting)
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

// TestShouldSkipRemindersMultiTag は複数タグ併記時の precedence を確認する:
// - #ignore が含まれていれば全 rt で skip（最優先）
// - それ以外は most-permissive（いずれかのタグが送信を望めば送信）
func TestShouldSkipRemindersMultiTag(t *testing.T) {
	allTypes := []ReminderType{RTRSVP, RTFinalCall, RTCondition, RTEquipment}

	// #練習 #sponsor: 練習が全リマインダを望む → どの rt でも skip しない
	practiceSponsor := Event{}
	practiceSponsor.Google.Title = "#練習 #sponsor 合同練習"
	for _, rt := range allTypes {
		if practiceSponsor.ShouldSkipReminders(rt) {
			t.Errorf("#練習 #sponsor ShouldSkipReminders(%q) = true, want false (練習相当で全送信)", rt)
		}
	}

	// #ignore #練習: ignore 最優先 → 全 rt で skip
	ignorePractice := Event{}
	ignorePractice.Google.Title = "#ignore #練習 テスト"
	for _, rt := range allTypes {
		if !ignorePractice.ShouldSkipReminders(rt) {
			t.Errorf("#ignore #練習 ShouldSkipReminders(%q) = false, want true (ignore 最優先で全 skip)", rt)
		}
	}

	// #meeting #event: RTRSVP のみ送信、それ以外は skip
	meetingEvent := Event{}
	meetingEvent.Google.Title = "#meeting #event 総会"
	if meetingEvent.ShouldSkipReminders(RTRSVP) {
		t.Errorf("#meeting #event ShouldSkipReminders(RTRSVP) = true, want false (RSVP は送信)")
	}
	for _, rt := range []ReminderType{RTFinalCall, RTCondition, RTEquipment} {
		if !meetingEvent.ShouldSkipReminders(rt) {
			t.Errorf("#meeting #event ShouldSkipReminders(%q) = false, want true (非 RSVP は skip)", rt)
		}
	}
}
