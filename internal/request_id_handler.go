package internal

import (
	"net/http"
	"uuid"
)

const maxLoggableRequestIDLength = 255

func NewRequestIDHandler(trustClientHeader bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trustClientHeader || r.Header.Get("X-Request-ID") == "" {
			r.Header.Set("X-Request-ID", uuid.New().String())
		}
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		next.ServeHTTP(w, r)
	})
}

func loggableRequestID(r *http.Request) string {
	id := r.Header.Get("X-Request-ID")
	if len(id) > maxLoggableRequestIDLength {
		id = id[:maxLoggableRequestIDLength]
	}
	return id
}
