package models

import (
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

func CreateEventFromCalendarAPI(cal *calendar.Event) GoogleEvent {
	must := func(t time.Time, err error) int64 {
		if err != nil {
			panic(err)
		}
		return t.Unix() * 1000 // ミリ秒にする
	}
	return GoogleEvent{
		ID:          cal.Id,
		Title:       cal.Summary,
		Description: cal.Description,
		StartTime:   must(time.Parse(time.RFC3339, cal.Start.DateTime)),
		EndTime:     must(time.Parse(time.RFC3339, cal.End.DateTime)),
		Location:    cal.Location,
	}
}
