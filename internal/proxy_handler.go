package internal

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func NewProxyHandler(targetUrl *url.URL, badGatewayPage string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.ErrorHandler = ProxyErrorHandler(badGatewayPage)

	return proxy
}

func ProxyErrorHandler(badGatewayPage string) func(w http.ResponseWriter, r *http.Request, err error) {
	content, err := os.ReadFile(badGatewayPage)
	if err != nil {
		slog.Debug("No custom 502 page found", "path", badGatewayPage)
		content = nil
	}

	return func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Info("Unable to proxy request", "path", r.URL.Path, "error", err)

		if errors.Is(err, ErrRequestBodyTooLarge) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if content != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadGateway)
			w.Write(content)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	}
}
