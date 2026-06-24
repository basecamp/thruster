package internal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyErrorHandler_clientCancellationReturnsClientClosedRequest(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler(w, r, context.Canceled)

	assert.Equal(t, StatusClientClosedRequest, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestProxyErrorHandler_wrappedClientCancellationReturnsClientClosedRequest(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler(w, r, fmt.Errorf("proxying request: %w", context.Canceled))

	assert.Equal(t, StatusClientClosedRequest, w.Code)
}

func TestProxyErrorHandler_clientCancellationIsNotLoggedAsProxyError(t *testing.T) {
	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(original)

	handler := ProxyErrorHandler("")
	handler(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), context.Canceled)

	assert.NotContains(t, buf.String(), "Unable to proxy request")
}

func TestProxyErrorHandler_upstreamErrorReturnsBadGateway(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler(w, r, errors.New("dial tcp [::1]:3000: connect: connection refused"))

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestProxyErrorHandler_connectionRefusedReturnsBadGateway(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	// A real connection-refused error is a net.Error, but it is not a timeout,
	// so it must still be treated as a bad gateway rather than a 504.
	err := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}
	require.False(t, err.Timeout())

	handler(w, r, err)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestProxyErrorHandler_upstreamTimeoutReturnsGatewayTimeout(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	// context.DeadlineExceeded satisfies net.Error with Timeout() == true, the
	// same shape the transport returns when an upstream read/dial times out.
	handler(w, r, context.DeadlineExceeded)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)
}

func TestProxyErrorHandler_chunkedEncodingErrorReturnsBadRequest(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler(w, r, errors.New("malformed chunked encoding"))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProxyErrorHandler_entityTooLargeReturnsRequestEntityTooLarge(t *testing.T) {
	handler := ProxyErrorHandler("")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	handler(w, r, &http.MaxBytesError{})

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// End-to-end: a client that disconnects mid-request must be recorded as a
// client-closed request (499), not an upstream failure (502).
func TestProxyHandler_clientDisconnectIsRecordedAsClientClosedRequest(t *testing.T) {
	upstreamReached := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamReached)
		<-r.Context().Done() // block until the client goes away
	}))
	defer upstream.Close()

	targetUrl, err := url.Parse(upstream.URL)
	require.NoError(t, err)

	proxy := NewProxyHandler(targetUrl, "", false)

	var capturedStatus int
	done := make(chan struct{})
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		proxy.ServeHTTP(recorder, r)
		capturedStatus = recorder.status
		close(done)
	}))
	defer front.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", front.URL, nil)
	require.NoError(t, err)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.DefaultClient.Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		_ = err // the client cancels the request, so an error is expected
	}()

	<-upstreamReached
	cancel()
	<-done
	<-clientDone

	assert.Equal(t, StatusClientClosedRequest, capturedStatus)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
