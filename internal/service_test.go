package internal

import (
	"errors"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gracefulShutdown must stop the HTTP server accepting new connections before
// it relays the termination signal to the upstream process. Otherwise there is
// a window during shutdown where we accept connections that the upstream --
// already signalled to stop -- can no longer serve, which surfaces to clients
// as 502s.
func TestGracefulShutdownDrainsServerBeforeSignalingUpstream(t *testing.T) {
	config := &Config{HttpDrainTimeout: 5 * time.Second}
	server, address := serveOnRandomPort(t, config, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Stop()

	require.True(t, accepting(address), "server should be accepting connections before shutdown")

	upstream := &probingSignaler{probeAddress: address}
	gracefulShutdown(server, upstream, syscall.SIGTERM)

	require.True(t, upstream.signalled, "upstream should have been signalled")
	assert.Equal(t, syscall.SIGTERM, upstream.signal)
	assert.False(t, upstream.acceptingWhenSignalled,
		"HTTP listener must be closed before the upstream is signalled")
}

// Even if relaying the signal to the upstream fails, the HTTP server must still
// have been drained.
func TestGracefulShutdownDrainsServerEvenWhenSignalingUpstreamFails(t *testing.T) {
	config := &Config{HttpDrainTimeout: 5 * time.Second}
	server, address := serveOnRandomPort(t, config, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Stop()

	upstream := &probingSignaler{probeAddress: address, err: errors.New("os: process already finished")}
	gracefulShutdown(server, upstream, syscall.SIGTERM)

	assert.True(t, upstream.signalled)
	assert.False(t, accepting(address), "server should be drained even when signalling the upstream fails")
}

// handleSignals must release its signal handler and return when the upstream
// exits on its own (no termination signal), rather than leaking a goroutine
// that keeps intercepting signals for the rest of the process's life.
func TestHandleSignalsReturnsWhenUpstreamExitsWithoutSignal(t *testing.T) {
	upstream := &UpstreamProcess{Started: make(chan struct{}, 1)}
	upstream.Started <- struct{}{} // the upstream has started

	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		(&Service{}).handleSignals(nil, upstream, stopped)
		close(done)
	}()

	close(stopped) // the upstream exited without a termination signal

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleSignals did not return after the upstream stopped")
	}
}

// serveOnRandomPort starts the server on an OS-assigned port using its own
// listener, so the bound address is known without a free-port-then-rebind race.
func serveOnRandomPort(t *testing.T, config *Config, handler http.Handler) (*Server, string) {
	t.Helper()

	server := NewServer(config, handler)
	server.httpServer = server.defaultHttpServer("127.0.0.1:0")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.httpServer.Serve(listener) }()

	return server, listener.Addr().String()
}

// probingSignaler records, at the moment it is signalled, whether the HTTP
// front-end is still accepting connections.
type probingSignaler struct {
	probeAddress           string
	err                    error
	signalled              bool
	signal                 os.Signal
	acceptingWhenSignalled bool
}

func (p *probingSignaler) Signal(sig os.Signal) error {
	p.signalled = true
	p.signal = sig
	p.acceptingWhenSignalled = accepting(p.probeAddress)
	return p.err
}

func accepting(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
