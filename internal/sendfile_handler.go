package internal

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
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

func (w *sendfileWriter) sendingFilename() string {
	return w.w.Header().Get("X-Sendfile")
}

func (w *sendfileWriter) serveFile(filename string) {
	slog.Debug("X-Sendfile sending file", "path", filename)

	w.setContentLength(filename)
	http.ServeFile(w.w, w.r, filename)
}

func (w *sendfileWriter) setContentLength(filename string) {
	// In most cases, `http.ServeFile` will set this for us. However, it will not
	// set it if the response also has a `Content-Encoding`.
	// (https://github.com/golang/go/commit/fdc21f3eafe94490e55e0bf018490b3aa9ba2383)
	//
	// If we don't set (or at least clear) the header in that case, we'll pass
	// through the `Content-Length` of the upstream's response, which can lead to
	// us serving an incomplete response.
	//
	// In particular, this happens when Rails is serving a gzipped asset via
	// `X-Sendfile`, which it does using `Content-Encoding: gzip` and
	// `Content-Length: 0`.

	fi, err := os.Stat(filename)
	if err != nil {
		w.w.Header().Del("Content-Length")
	} else {
		w.w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	}
}
