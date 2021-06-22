package filters

import (
	"net/http"
	"os"

	"github.com/otiai10/marmoset"
)

type AuthFilter struct {
	marmoset.Filter
}

func (auth *AuthFilter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()

	if !ok {
		w.Header().Add("WWW-Authenticate", `Basic realm="Username/Password required"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if u != os.Getenv("HUB_BASICAUTH_USERNAME") && p != os.Getenv("HUB_BASICAUTH_PASSWORD") {
		w.Header().Add("WWW-Authenticate", `Basic realm="Username/Password required"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	auth.Next.ServeHTTP(w, r)
}
