package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServeError(t *testing.T) {
	w := httptest.NewRecorder()

	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Etag", `"abc"`)
	w.Header().Set("Expires", "Thu, 01 Jan 2026 00:00:00 GMT")
	w.Header().Set("Last-Modified", "Thu, 01 Jan 2026 00:00:00 GMT")
	w.Header().Set("Content-Length", "12345")
	w.Header().Set("Content-Type", "application/pdf")

	w.Header().Set("Set-Cookie", "session=abc")
	w.Header().Set("X-Request-ID", "req-1")
	w.Header().Set("X-Frame-Options", "DENY")

	serveError(w, http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "Not Found\n", w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")

	for _, name := range representationHeaders {
		assert.Empty(t, w.Header().Get(name), name)
	}
	assert.Empty(t, w.Header().Get("Content-Length"))

	assert.Equal(t, "session=abc", w.Header().Get("Set-Cookie"))
	assert.Equal(t, "req-1", w.Header().Get("X-Request-ID"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}
