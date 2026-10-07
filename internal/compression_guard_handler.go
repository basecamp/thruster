package internal

import (
	"bufio"
	"net"
	"net/http"
	"slices"

	"github.com/klauspost/compress/gzhttp"
)

func NewCompressionGuardHandler(next http.Handler) http.Handler {
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
	return len(r.Header.Values("Cookie")) > 0 ||
		len(r.Header.Values("Authorization")) > 0 ||
		len(r.Header.Values("X-Csrf-Token")) > 0
}

type compressionGuardResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *compressionGuardResponseWriter) WriteHeader(statusCode int) {
	if statusCode >= 100 && statusCode < 200 {
		w.ResponseWriter.WriteHeader(statusCode)
		return
	}

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
	if len(h.Values("Set-Cookie")) > 0 {
		return true
	}

	cc := parseCacheControl(h)
	if cc.malformed || cc.has("private") || cc.has("no-store") {
		return true
	}

	return slices.Contains(varyNames(h), "Cookie")
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
