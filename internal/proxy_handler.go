package internal

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
)

func NewProxyHandler(targetUrl *url.URL, badGatewayPage string) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetUrl)
			r.Out.Host = r.In.Host
			r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
			r.SetXForwarded()
		},
		ErrorHandler: ProxyErrorHandler(badGatewayPage),
		Transport:    createProxyTransport(),
		Director:     CustomDirector(targetUrl, badGatewayPage),
	}
}

func CustomDirector(targetUrl *url.URL, badGatewayPage string) func(req *http.Request) {
    return func(req *http.Request) {
        filePath := filepath.Join("public", req.URL.Path)
        if _, err := os.Stat(filePath); err == nil {
            http.ServeFile(req.Response, req, filePath)
            return
        }

        req.URL.Scheme = targetUrl.Scheme
        req.URL.Host = targetUrl.Host
        req.Host = targetUrl.Host
    }
}

func ProxyErrorHandler(badGatewayPage string) func(w http.ResponseWriter, r *http.Request, err error) {
    content, err := os.ReadFile(badGatewayPage)
    if err != nil {
        slog.Debug("No custom 502 page found", "path", badGatewayPage)
        content = nil
    }

    return func(w http.ResponseWriter, r *http.Request, err error) {
        slog.Info("Unable to proxy request", "path", r.URL.Path, "error", err)

        if isRequestEntityTooLarge(err) {
            w.WriteHeader(http.StatusRequestEntityTooLarge)
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

func isRequestEntityTooLarge(err error) bool {
    var maxBytesError *http.MaxBytesError
    return errors.As(err, &maxBytesError)
}

func createProxyTransport() *http.Transport {
    transport := http.DefaultTransport.(*http.Transport).Clone()
    transport.DisableCompression = true

    return transport
}
