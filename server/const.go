package server

import (
	"os"
	"time"
)

var (
	SessionCookieName = os.Getenv("BROWSER_SESSION_KEY_ID")
	HubHelpPageURL    = os.Getenv("HUB_HELP_PAGE_URL")

	ServerSessionExpire = time.Hour * 24 * 14

	ServiceLocation, _ = time.LoadLocation("Asia/Tokyo")
)

func HubBaseURL() string {
	return os.Getenv("HUB_WEBPAGE_BASE_URL")
}

func HubConditioningCheckSheetURL() string {
	return os.Getenv("HUB_CONDITIONING_CHECK_SHEET_URL")
}
