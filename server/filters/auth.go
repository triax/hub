package filters

import (
	"net/http"

	"github.com/otiai10/marmoset"
)

type AuthFilter struct {
	marmoset.Filter
}

// TODO: Basic authやめる
func (auth *AuthFilter) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	if req.URL.Query().Get("u") == "foo" && req.URL.Query().Get("p") == "baa" {
		auth.Next.ServeHTTP(w, req)
	} else {
		http.Redirect(w, req, "/login", http.StatusTemporaryRedirect)
	}
}
