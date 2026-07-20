package internal

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// StatusClientClosedRequest is a non-standard status code, following nginx
// convention, used when the client disconnects before we are able to respond.
// Recording it (rather than a 502) keeps client-cancelled requests out of the
// 5xx bucket in access logs and metrics.
const StatusClientClosedRequest = 499

func NewProxyHandler(targetUrl *url.URL, badGatewayPage string, forwardHeaders bool) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetUrl)
			r.Out.Host = r.In.Host
			setXForwarded(r, forwardHeaders)
		},
		ErrorHandler: ProxyErrorHandler(badGatewayPage),
		Transport:    createProxyTransport(),
	}
}

func ProxyErrorHandler(badGatewayPage string) func(w http.ResponseWriter, r *http.Request, err error) {
	content, err := os.ReadFile(badGatewayPage)
	if err != nil {
		slog.Debug("No custom 502 page found", "path", badGatewayPage)
		content = nil
	}

	return func(w http.ResponseWriter, r *http.Request, err error) {
		if isClientCancellation(err) {
			// The client disconnected before we could respond, so there is
			// nothing to send. We still set a status code so the request is
			// recorded in the access log, but as a client-closed request
			// rather than an upstream failure -- otherwise client cancellations
			// (a fetch aborted, a Turbo Frame swapped, a navigation away) show
			// up as 502s and pollute error dashboards.
			slog.Debug("Client disconnected before response", "path", r.URL.Path)
			w.WriteHeader(StatusClientClosedRequest)
			return
		}

		slog.Info("Unable to proxy request", "path", r.URL.Path, "error", err)

		if isRequestEntityTooLarge(err) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		if isGatewayTimeout(err) {
			w.WriteHeader(http.StatusGatewayTimeout)
			return
		}

		if isChunkedEncodingError(err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if content != nil {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write(content)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	}
}

func setXForwarded(r *httputil.ProxyRequest, forwardHeaders bool) {
	if forwardHeaders {
		r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
	}

	r.SetXForwarded()

	if forwardHeaders {
		// Preserve original headers if we had them
		if r.In.Header.Get("X-Forwarded-Host") != "" {
			r.Out.Header.Set("X-Forwarded-Host", r.In.Header.Get("X-Forwarded-Host"))
		}
		if r.In.Header.Get("X-Forwarded-Proto") != "" {
			r.Out.Header.Set("X-Forwarded-Proto", r.In.Header.Get("X-Forwarded-Proto"))
		}
	}
}

func isClientCancellation(err error) bool {
	return errors.Is(err, context.Canceled)
}

func isRequestEntityTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func isGatewayTimeout(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

func isChunkedEncodingError(err error) bool {
	if err == nil {
		return false
	}

	// The chunked encoding support in the stdlib returns these failures as
	// plain errors built with errors.New, so matching them means string
	// matching on the error message, unfortunately.
	switch err.Error() {
	case "invalid byte in chunk length",
		"http chunk length too large",
		"malformed chunked encoding",
		"trailer header without chunked transfer encoding",
		"too many trailers":
		return true
	}

	return false
}

func createProxyTransport() *http.Transport {
	// The default transport requests compressed responses even if the client
	// didn't. If it receives a compressed response but the client wants
	// uncompressed, the transport decompresses the response transparently.
	//
	// Although that seems helpful, it doesn't play well with X-Sendfile
	// responses, as it may result in us being handed a reference to a file on
	// disk that is already compressed, and we'd have to similarly decompress it
	// before serving it to the client. This is wasteful, especially since there
	// was probably an uncompressed version of it on disk already. It's also a bit
	// fiddly to do on the fly without the ability to seek around in the
	// uncompressed content.
	//
	// Compression between us and the upstream server is likely to be of limited
	// use anyway, since we're only proxying from localhost. Given that fact --
	// and the fact that most clients *will* request compressed responses anyway,
	// which makes all of this moot -- our best option is to disable this
	// on-the-fly compression.

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true

	return transport
}
