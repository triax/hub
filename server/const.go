package server

import (
	"os"
	"time"
)

var (
	SessionCookieName = os.Getenv("BROWSER_SESSION_KEY_ID")
	HubBaseURL        = os.Getenv("HUB_WEBPAGE_BASE_URL") // without following slash
	HubHelpPageURL    = os.Getenv("HUB_HELP_PAGE_URL")

	ServerSessionExpire = time.Hour * 24 * 14
)
