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
	go server.Serve(listener)

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
