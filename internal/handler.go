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
	imageProxyEnabled        bool
}

func NewHandler(options HandlerOptions) http.Handler {
	mux := http.NewServeMux()

	handler := NewProxyHandler(options.targetUrl, options.badGatewayPage)
	handler = NewCacheHandler(options.cache, options.maxCacheableResponseBody, handler)
	handler = NewSendfileHandler(options.xSendfileEnabled, handler)
	handler = gzhttp.GzipHandler(handler)
	handler = NewMaxRequestBodyHandler(options.maxRequestBody, handler)
	handler = NewLoggingMiddleware(slog.Default(), handler)

	if options.imageProxyEnabled {
		RegisterNewImageProxyHandler(mux)
	}

	mux.Handle("/", handler)

	return mux
}
