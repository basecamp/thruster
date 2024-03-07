package internal

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/klauspost/compress/gzhttp"
)

type HandlerOptions struct {
	badGatewayPage           string
	cache                    Cache
	maxCacheableResponseBody int
	maxRequestBody           int
	targetUrl                *url.URL
	xSendfileEnabled         bool
}

func NewHandler(options HandlerOptions) http.Handler {
	handler := NewProxyHandler(options.targetUrl, options.badGatewayPage)
	handler = NewCacheHandler(options.cache, options.maxCacheableResponseBody, handler)
	handler = NewSendfileHandler(options.xSendfileEnabled, handler)
	handler = gzhttp.GzipHandler(handler)

	if options.maxRequestBody > 0 {
		handler = http.MaxBytesHandler(handler, int64(options.maxRequestBody))
	}

	handler = NewLoggingMiddleware(slog.Default(), handler)

	return handler
}
