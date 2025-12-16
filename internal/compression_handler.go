package internal

import (
	"net/http"

	"github.com/klauspost/compress/gzhttp"
)

func NewCompressionHandler(jitter int, disableOnAuth bool, next http.Handler) http.Handler {
	var wrapper func(http.Handler) http.HandlerFunc
	var err error

	if jitter > 0 {
		wrapper, err = gzhttp.NewWrapper(
			gzhttp.MinSize(1024),
			gzhttp.CompressionLevel(6),
			gzhttp.RandomJitter(jitter, 0, false),
		)
	} else {
		wrapper, err = gzhttp.NewWrapper(
			gzhttp.MinSize(1024),
			gzhttp.CompressionLevel(6),
		)
	}

	if err != nil {
		panic("failed to create gzip wrapper: " + err.Error())
	}

	handler := wrapper(next)

	if disableOnAuth {
		return NewCompressionGuardHandler(handler)
	}

	return handler
}
