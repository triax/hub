package server

import "os"

var SessionCookieName = os.Getenv("BROWSER_SESSION_KEY_ID")
