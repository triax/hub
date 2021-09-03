package models

type (
	GoogleEvent struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		StartTime   int64  `json:"start_time"`
		EndTime     int64  `json:"end_time"`
		Location    string `json:"location"`
	}
)
