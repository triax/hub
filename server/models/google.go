package models

import (
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
)

type (
	GoogleEvent struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		StartTime   int64  `json:"start_time"` // ミリ秒
		EndTime     int64  `json:"end_time"`
		Location    string `json:"location"`
	}
)

func (e GoogleEvent) Start() time.Time { // time.Timeへのコンバータ
	return time.Unix(e.StartTime/1000, 0)
}

// parseCalendarTime は Google Calendar API の EventDateTime からミリ秒UNIXタイムスタンプを返す。
// 終日イベントの場合 DateTime が空で Date ("2006-01-02") が入るため、両方を考慮する。
func parseCalendarTime(edt *calendar.EventDateTime) (int64, error) {
	if edt.DateTime != "" {
		t, err := time.Parse(time.RFC3339, edt.DateTime)
		if err != nil {
			return 0, err
		}
		return t.Unix() * 1000, nil
	}
	if edt.Date != "" {
		t, err := time.Parse("2006-01-02", edt.Date)
		if err != nil {
			return 0, err
		}
		return t.Unix() * 1000, nil
	}
	return 0, fmt.Errorf("both DateTime and Date are empty")
}

func CreateEventFromCalendarAPI(cal *calendar.Event) (GoogleEvent, error) {
	startMs, err := parseCalendarTime(cal.Start)
	if err != nil {
		return GoogleEvent{}, fmt.Errorf("start time parse error: %w", err)
	}
	endMs, err := parseCalendarTime(cal.End)
	if err != nil {
		return GoogleEvent{}, fmt.Errorf("end time parse error: %w", err)
	}
	return GoogleEvent{
		ID:          cal.Id,
		Title:       cal.Summary,
		Description: cal.Description,
		StartTime:   startMs,
		EndTime:     endMs,
		Location:    cal.Location,
	}, nil
}
