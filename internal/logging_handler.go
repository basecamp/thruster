package internal

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type LoggingHandler struct {
	logger *slog.Logger
	next   http.Handler
}

func NewLoggingHandler(logger *slog.Logger, next http.Handler) *LoggingHandler {
	return &LoggingHandler{
		logger: logger,
		next:   next,
	}
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	writer := newResponseWriter(w)

	started := time.Now()
	h.next.ServeHTTP(writer, r)
	elapsed := time.Since(started)

	userAgent := r.Header.Get("User-Agent")
	reqContent := r.Header.Get("Content-Type")
	respContent := writer.Header().Get("Content-Type")
	cache := writer.Header().Get("X-Cache")
	remoteAddr := r.Header.Get("X-Forwarded-For")
	if remoteAddr == "" {
		remoteAddr = r.RemoteAddr
	}

	h.logger.Info("Request",
		"path", r.URL.Path,
		"status", writer.statusCode,
		"dur", elapsed.Milliseconds(),
		"method", r.Method,
		"req_content_length", r.ContentLength,
		"req_content_type", reqContent,
		"resp_content_length", writer.bytesWritten,
		"resp_content_type", respContent,
		"remote_addr", remoteAddr,
		"user_agent", userAgent,
		"cache", cache,
		"query", r.URL.RawQuery,
		"proto", r.Proto)
}

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK, 0}
}

// WriteHeader is used to capture the status code
func (r *responseWriter) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write is used to capture the amount of data written
func (r *responseWriter) Write(b []byte) (int, error) {
	bytesWritten, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(bytesWritten)
	return bytesWritten, err
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("ResponseWriter does not implement http.Hijacker")
	}

	con, rw, err := hijacker.Hijack()
	if err == nil {
		r.statusCode = http.StatusSwitchingProtocols
	}
	return con, rw, err
}

func (r *responseWriter) Flush() {
	flusher, ok := r.ResponseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}
