package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestServerDefaultProtocols(t *testing.T) {
	config, err := NewConfig()
	require.NoError(t, err)

	server := NewServer(config, nil)

	s := server.defaultHttpServer(":0")

	assert.True(t, s.Protocols.HTTP1())
	assert.True(t, s.Protocols.HTTP2())
	assert.False(t, s.Protocols.UnencryptedHTTP2())
}

func TestServerEnabledH2CWhenConfigProvided(t *testing.T) {
	usingEnvVar(t, "H2C_ENABLED", "true")

	config, err := NewConfig()
	require.NoError(t, err)

	server := NewServer(config, nil)

	s := server.defaultHttpServer(":0")

	assert.True(t, s.Protocols.HTTP1())
	assert.True(t, s.Protocols.HTTP2())
	assert.True(t, s.Protocols.UnencryptedHTTP2())
}

func TestServerCanMakeAnEndToEndH2CRequestWhenEnabled(t *testing.T) {
	resp, err := makeRoundTripH2cRequest(t, true)
	require.NoError(t, err)

	assert.Equal(t, "HTTP/2.0", resp.Proto)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServerDefaultCannotMakeH2CRequest(t *testing.T) {
	_, err := makeRoundTripH2cRequest(t, false)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "http2: failed reading the frame payload")
}

func TestHttpRedirectHandlerRejectsSpoofedHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://evil.example.com/path", nil)
	req.Host = "evil.example.com"
	recorder := httptest.NewRecorder()

	handler := httpRedirectHandler([]string{"legit.example.com"})
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMisdirectedRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Misdirected Request")
	assert.Empty(t, recorder.Header().Get("Location"))
}

func TestHttpRedirectHandlerAllowedDomainGets301(t *testing.T) {
	req := httptest.NewRequest("GET", "http://legit.example.com/path?q=1", nil)
	req.Host = "legit.example.com"
	recorder := httptest.NewRecorder()

	handler := httpRedirectHandler([]string{"legit.example.com"})
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://legit.example.com/path?q=1", recorder.Header().Get("Location"))
}

func TestHttpRedirectHandlerAllowedDomainWithPort(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com:9877/path", nil)
	req.Host = "example.com:9877"
	recorder := httptest.NewRecorder()

	handler := httpRedirectHandler([]string{"example.com"})
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://example.com/path", recorder.Header().Get("Location"))
}

func TestHttpRedirectHandlerMultipleTLSDomains(t *testing.T) {
	req := httptest.NewRequest("GET", "http://second.example.com/", nil)
	req.Host = "second.example.com"
	recorder := httptest.NewRecorder()

	handler := httpRedirectHandler([]string{"first.example.com", "second.example.com"})
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://second.example.com/", recorder.Header().Get("Location"))
}

func TestHttpRedirectHandlerCaseInsensitiveMatch(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Host = "example.com"
	recorder := httptest.NewRecorder()

	handler := httpRedirectHandler([]string{"Example.COM"})
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://example.com/", recorder.Header().Get("Location"))
}

func TestHttpRedirectHandlerIDNDomain(t *testing.T) {
	// Configure with a unicode domain; the handler normalizes it to Punycode
	// for comparison, matching autocert.HostWhitelist behavior.
	handler := httpRedirectHandler([]string{"\u00fc\u00f6\u00e4.example.com"})

	t.Run("unicode host matches unicode config", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://xn--4ca9ar.example.com/", nil)
		req.Host = "\u00fc\u00f6\u00e4.example.com"
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
		assert.Equal(t, "https://xn--4ca9ar.example.com/", recorder.Header().Get("Location"))
	})

	t.Run("punycode host matches unicode config", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://xn--4ca9ar.example.com/", nil)
		req.Host = "xn--4ca9ar.example.com"
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
		assert.Equal(t, "https://xn--4ca9ar.example.com/", recorder.Header().Get("Location"))
	})
}

func TestHttpRedirectHandlerRejectsHostFailingIDNANormalization(t *testing.T) {
	handler := httpRedirectHandler([]string{"legit.example.com"})

	// A leading hyphen in a label violates IDNA2008 label validation rules,
	// causing idna.Lookup.ToASCII to return an error.
	req := httptest.NewRequest("GET", "http://legit.example.com/", nil)
	req.Host = "-invalid-.example.com"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusMisdirectedRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Misdirected Request")
	assert.Empty(t, recorder.Header().Get("Location"))
}

func makeRoundTripH2cRequest(t *testing.T, h2cEnabled bool) (*http.Response, error) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "HTTP/1.1", r.Proto, "The upstream should still be serving http/1.1")
	}))
	defer upstream.Close()

	config, err := NewConfig()
	require.NoError(t, err)

	config.H2CEnabled = h2cEnabled

	h := NewHandler(handlerOptions(upstream.URL))
	s := NewServer(config, nil)

	server := s.defaultHttpServer(":0")
	server.Handler = h
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	go func() { assert.NoError(t, server.Serve(listener)) }()

	client := http.Client{
		// Force the http.Client to use an http/2 connection over cleartext.
		Transport: &http2.Transport{
			// Allow non-TLS requests.
			AllowHTTP: true,
			// When dialing, ignore the TLS config.
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}

	return client.Get(fmt.Sprintf("http://%s/", listener.Addr()))
}
