package internal

import (
	"bufio"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
)

var errNotRegularFile = errors.New("not a regular file")

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
	filename := w.sendingFilename()
	w.w.Header().Del("X-Sendfile")

	w.sendingFile = filename != ""
	w.headerWritten = true

	if w.sendingFile {
		w.serveFile(filename)
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

func (w *sendfileWriter) Flush() {
	flusher, ok := w.w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (w *sendfileWriter) sendingFilename() string {
	return w.w.Header().Get("X-Sendfile")
}

func (w *sendfileWriter) serveFile(filename string) {
	slog.Debug("X-Sendfile sending file", "request_id", loggableRequestID(w.r), "path", filename)

	f, err := os.Open(filename)
	if err != nil {
		w.serveFileError(filename, err)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		w.serveFileError(filename, err)
		return
	}

	if !fi.Mode().IsRegular() {
		w.serveFileError(filename, errNotRegularFile)
		return
	}

	w.w.Header().Del("Content-Length")

	http.ServeContent(&contentLengthWriter{w.w, fi.Size()}, w.r, fi.Name(), fi.ModTime(), f)
}

func (w *sendfileWriter) serveFileError(filename string, err error) {
	slog.Info("Unable to serve X-Sendfile file", "request_id", loggableRequestID(w.r), "path", w.r.URL.Path, "file", filename, "error", err)

	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR), errors.Is(err, errNotRegularFile):
		status = http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		status = http.StatusForbidden
	}

	serveError(w.w, status)
}

type contentLengthWriter struct {
	http.ResponseWriter
	size int64
}

func (w *contentLengthWriter) WriteHeader(code int) {
	if code == http.StatusOK && w.Header().Get("Content-Length") == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(w.size, 10))
	}

	w.ResponseWriter.WriteHeader(code)
}
