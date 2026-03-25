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

func TestHttpRedirect(t *testing.T) {
	s := &Server{
		config: &Config{
			TLSDomains:  []string{"example.com", "café.example.com"},
			StoragePath: t.TempDir(),
		},
	}
	s.manager = s.certManager()

	redirect := func(url string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		s.httpRedirectHandler(w, httptest.NewRequest("GET", url, nil))
		return w
	}

	t.Run("disallowed host", func(t *testing.T) {
		w := redirect("http://evil.com/path")

		assert.Equal(t, http.StatusMisdirectedRequest, w.Code)
	})

	t.Run("allowed host", func(t *testing.T) {
		w := redirect("http://example.com/path")

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "https://example.com/path", w.Header().Get("Location"))
	})

	t.Run("allowed host with explicit port", func(t *testing.T) {
		w := redirect("http://example.com:80/path")

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "https://example.com/path", w.Header().Get("Location"))
	})

	t.Run("mixed case allowed host", func(t *testing.T) {
		w := redirect("http://Example.COM/path")

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "https://example.com/path", w.Header().Get("Location"))
	})

	t.Run("unicode host", func(t *testing.T) {
		w := redirect("http://café.example.com/path")

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "https://xn--caf-dma.example.com/path", w.Header().Get("Location"))
	})

	t.Run("unicode host using punycode", func(t *testing.T) {
		w := redirect("http://xn--caf-dma.example.com/path")

		assert.Equal(t, http.StatusMovedPermanently, w.Code)
		assert.Equal(t, "https://xn--caf-dma.example.com/path", w.Header().Get("Location"))
	})

	t.Run("unicode host using invalid punycode", func(t *testing.T) {
		w := redirect("http://-xn--caf-dma.example.com/path")

		assert.Equal(t, http.StatusMisdirectedRequest, w.Code)
	})
}
