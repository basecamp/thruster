package internal

import (
	"fmt"
	"net/http"
	"time"
)

func NewRequestStartMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp := time.Now().UnixMilli()
		r.Header.Set("X-Request-Start", fmt.Sprintf("t=%d", timestamp))
		next.ServeHTTP(w, r)
	})
}