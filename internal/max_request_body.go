package internal

import (
	"errors"
	"io"
	"net/http"
)

func NewMaxRequestBodyHandler(maxBytes int, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = newLimitedReadCloser(r.Body, maxBytes)
		handler.ServeHTTP(w, r)
	})
}

// Private

var ErrRequestBodyTooLarge = errors.New("request body too large")

type limitedReadCloser struct {
	reader io.ReadCloser
	io.Closer
	bytesRemaining int
}

func newLimitedReadCloser(reader io.ReadCloser, maxBytes int) io.ReadCloser {
	if maxBytes == 0 {
		return reader // 0 means unlimited
	}

	return &limitedReadCloser{
		reader:         reader,
		Closer:         reader,
		bytesRemaining: maxBytes,
	}
}

func (r *limitedReadCloser) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytesRemaining -= n

	if r.bytesRemaining < 0 {
		return 0, ErrRequestBodyTooLarge
	}

	return n, err
}
