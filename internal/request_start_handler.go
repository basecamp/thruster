package internal

import (
	"fmt"
	"net/http"
	"time"
)

func NewRequestStartHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-Start") == "" {
			timestamp := time.Now().UnixMilli()
			r.Header.Set("X-Request-Start", fmt.Sprintf("t=%d", timestamp))
		}
		next.ServeHTTP(w, r)
	})
}
