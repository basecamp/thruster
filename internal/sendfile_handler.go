package internal

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
)

type SendfileHandler struct {
	enabled bool
	next    http.Handler
}

func NewSendfileHandler(enabled bool, next http.Handler) *SendfileHandler {
	return &SendfileHandler{
		enabled: enabled,
		next:    next,
	}
}

func (h *SendfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.enabled {
		r.Header.Set("X-Sendfile-Type", "X-Sendfile")
		w = &sendfileWriter{w, r, false, false}
	} else {
		r.Header.Del("X-Sendfile-Type")
	}

	h.next.ServeHTTP(w, r)
}

type sendfileWriter struct {
	w             http.ResponseWriter
	r             *http.Request
	headerWritten bool
	sendingFile   bool
}

func (w *sendfileWriter) Header() http.Header {
	return w.w.Header()
}

func (w *sendfileWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}

	if w.sendingFile {
		return 0, http.ErrBodyNotAllowed
	}

	return w.w.Write(b)
}

func (w *sendfileWriter) WriteHeader(statusCode int) {
	fname := w.sendingFilename()
	w.sendingFile = fname != ""
	w.headerWritten = true

	w.w.Header().Del("X-Sendfile")

	if w.sendingFile {
		slog.Info("X-Sendfile sending file", "path", fname)
		http.ServeFile(w.w, w.r, fname)
	} else {
		w.w.WriteHeader(statusCode)
	}
}

func (w *sendfileWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.w.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("ResponseWriter does not implement http.Hijacker")
	}

	return hijacker.Hijack()
}

func (w *sendfileWriter) sendingFilename() string {
	return w.w.Header().Get("X-Sendfile")
}
