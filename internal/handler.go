package internal

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/klauspost/compress/gzhttp"
)

type HandlerOptions struct {
	badGatewayPage               string
	cache                        Cache
	maxCacheableResponseBody     int
	maxRequestBody               int
	targetUrl                    *url.URL
	xSendfileEnabled             bool
	gzipCompressionEnabled       bool
	gzipCompressionDisableOnAuth bool
	gzipCompressionJitter        int
	forwardHeaders               bool
	logRequests                  bool
}

func NewHandler(options HandlerOptions) http.Handler {
	handler := NewProxyHandler(options.targetUrl, options.badGatewayPage, options.forwardHeaders)
	handler = NewCacheHandler(options.cache, options.maxCacheableResponseBody, handler)
	handler = NewSendfileHandler(options.xSendfileEnabled, handler)
	handler = NewRequestStartMiddleware(handler)

	if options.gzipCompressionEnabled {
		var wrapper func(http.Handler) http.HandlerFunc
		var err error

		if options.gzipCompressionJitter > 0 {
			wrapper, err = gzhttp.NewWrapper(
				gzhttp.MinSize(1024),
				gzhttp.CompressionLevel(6),
				gzhttp.RandomJitter(options.gzipCompressionJitter, 0, false),
			)
		} else {
			wrapper, err = gzhttp.NewWrapper(
				gzhttp.MinSize(1024),
				gzhttp.CompressionLevel(6),
			)
		}

		if err != nil {
			// If we cannot create the wrapper with the requested configuration (including jitter),
			// we must fail hard rather than silently downgrading security or performance.
			panic("failed to create gzip wrapper: " + err.Error())
		}

		gzipHandler := wrapper(handler)

		if options.gzipCompressionDisableOnAuth {
			handler = NewCompressionGuardMiddleware(gzipHandler)
		} else {
			handler = gzipHandler
		}
	}

	if options.maxRequestBody > 0 {
		handler = http.MaxBytesHandler(handler, int64(options.maxRequestBody))
	}

	if options.logRequests {
		handler = NewLoggingMiddleware(slog.Default(), handler)
	}

	return handler
}
