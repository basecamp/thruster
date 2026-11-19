package internal

import (
	"net/http"
	"strings"
)

// Go drops query pairs containing a raw ";", so encode it to keep it as part of the value.
func NewQuerySemicolonHandler(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, ";") {
			next.ServeHTTP(w, r)
			return
		}

		r2 := new(http.Request)
		*r2 = *r
		u := *r.URL
		u.RawQuery = strings.ReplaceAll(r.URL.RawQuery, ";", "%3B")
		r2.URL = &u

		next.ServeHTTP(w, r2)
	})
}
