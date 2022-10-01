package server

import (
	"os"
	"time"
)

var (
	SessionCookieName = os.Getenv("BROWSER_SESSION_KEY_ID")
	HubBaseURL        = "https://hub.triax.football" // TODO: Make it configurable

	ServerSessionExpire = time.Hour * 24 * 14
)
