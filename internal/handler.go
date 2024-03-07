package internal

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/klauspost/compress/gzhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type HandlerOptions struct {
	badGatewayPage           string
	cache                    Cache
	maxCacheableResponseBody int
	maxRequestBody           int
	targetUrl                *url.URL
	xSendfileEnabled         bool
	gzipCompressionEnabled   bool
	forwardHeaders           bool
}

func NewHandler(options HandlerOptions) http.Handler {
	handler := NewProxyHandler(options.targetUrl, options.badGatewayPage, options.forwardHeaders)
	handler = NewCacheHandler(options.cache, options.maxCacheableResponseBody, handler)
	handler = NewSendfileHandler(options.xSendfileEnabled, handler)
	handler = h2c.NewHandler(handler, &http2.Server{})

	if options.gzipCompressionEnabled {
		handler = gzhttp.GzipHandler(handler)
	}

	if options.maxRequestBody > 0 {
		handler = http.MaxBytesHandler(handler, int64(options.maxRequestBody))
	}

	handler = NewLoggingMiddleware(slog.Default(), handler)

	return handler
}
