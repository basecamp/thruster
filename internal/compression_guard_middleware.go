package internal

import (
	"bufio"
	"net"
	"net/http"
	"strings"

	"github.com/klauspost/compress/gzhttp"
)

func NewCompressionGuardMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for user-specific headers in the request
		if hasUserSpecificRequestHeaders(r) {
			w.Header().Set(gzhttp.HeaderNoCompression, "1")
		}

		// Wrap the ResponseWriter to check for user-specific headers in the response
		wrappedWriter := &compressionGuardResponseWriter{ResponseWriter: w}
		next.ServeHTTP(wrappedWriter, r)
	})
}

func hasUserSpecificRequestHeaders(r *http.Request) bool {
	return r.Header.Get("Cookie") != "" ||
		r.Header.Get("Authorization") != "" ||
		r.Header.Get("X-Csrf-Token") != ""
}

type compressionGuardResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *compressionGuardResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true

	// Check for user-specific headers in the response
	if hasUserSpecificResponseHeaders(w.Header()) {
		w.Header().Set(gzhttp.HeaderNoCompression, "1")
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *compressionGuardResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func hasUserSpecificResponseHeaders(h http.Header) bool {
	if h.Get("Set-Cookie") != "" {
		return true
	}

	cacheControl := strings.ToLower(h.Get("Cache-Control"))
	for _, directive := range strings.Split(cacheControl, ",") {
		dir := strings.TrimSpace(directive)
		// Strip any value (e.g. private="Set-Cookie") before comparison.
		dirName := strings.SplitN(dir, "=", 2)[0]
		if dirName == "private" || dirName == "no-store" {
			return true
		}
	}

	vary := h.Get("Vary")
	for _, token := range strings.Split(vary, ",") {
		if strings.EqualFold(strings.TrimSpace(token), "cookie") {
			return true
		}
	}

	return false
}

// Flush implements http.Flusher
func (w *compressionGuardResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker
func (w *compressionGuardResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
