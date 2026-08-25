package internal

import "net/http"

var representationHeaders = []string{
	"Cache-Control",
	"Content-Disposition",
	"Content-Encoding",
	"Etag",
	"Expires",
	"Last-Modified",
}

func serveError(w http.ResponseWriter, status int) {
	for _, name := range representationHeaders {
		w.Header().Del(name)
	}

	http.Error(w, http.StatusText(status), status)
}
