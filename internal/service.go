package internal

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

type Service struct {
	config *Config
}

func NewService(config *Config) *Service {
	return &Service{
		config: config,
	}
}

func (s *Service) Run() int {
	handlerOptions := HandlerOptions{
		cache:                        s.cache(),
		targetUrl:                    s.targetUrl(),
		xSendfileEnabled:             s.config.XSendfileEnabled,
		gzipCompressionEnabled:       s.config.GzipCompressionEnabled,
		maxCacheableResponseBody:     s.config.MaxCacheItemSizeBytes,
		maxRequestBody:               s.config.MaxRequestBody,
		badGatewayPage:               s.config.BadGatewayPage,
		forwardHeaders:               s.config.ForwardHeaders,
		logRequests:                  s.config.LogRequests,
		gzipCompressionDisableOnAuth: s.config.GzipCompressionDisableOnAuth,
		gzipCompressionJitter:        s.config.GzipCompressionJitter,
	}

	handler := NewHandler(handlerOptions)
	server := NewServer(s.config, handler)
	upstream := NewUpstreamProcess(s.config.UpstreamCommand, s.config.UpstreamArgs...)

	if err := server.Start(); err != nil {
		return 1
	}
	defer server.Stop()

	s.setEnvironment()

	stopped := make(chan struct{})
	go s.handleSignals(server, upstream, stopped)

	exitCode, err := upstream.Run()
	close(stopped)
	if err != nil {
		slog.Error("Failed to start wrapped process", "command", s.config.UpstreamCommand, "args", s.config.UpstreamArgs, "error", err)
		return 1
	}

	return exitCode
}

// Private

type signaler interface {
	Signal(os.Signal) error
}

// handleSignals waits for a termination signal and then shuts down. It only
// arms signal handling once the upstream process has started, so that a signal
// arriving during startup keeps the default behaviour of terminating the
// process rather than trying to signal an upstream that doesn't exist yet. It
// returns (releasing the signal handler) when the upstream exits without a
// signal, signalled by stopped being closed, so it doesn't outlive the service.
func (s *Service) handleSignals(server *Server, upstream *UpstreamProcess, stopped <-chan struct{}) {
	select {
	case <-upstream.Started:
	case <-stopped:
		return
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)

	select {
	case sig := <-ch:
		gracefulShutdown(server, upstream, sig)
	case <-stopped:
	}
}

// gracefulShutdown drains the HTTP server before relaying the signal to the
// upstream process. The order matters: draining first closes the listener so
// we stop accepting new connections and let in-flight requests finish, while
// the upstream is still able to serve them. Relaying the signal first would
// leave a window where we keep accepting connections but the upstream has
// already stopped listening, which surfaces to clients as 502s on every deploy.
func gracefulShutdown(server *Server, upstream signaler, sig os.Signal) {
	slog.Info("Draining HTTP server before relaying signal to upstream", "signal", sig.String())
	server.Stop()
	if err := upstream.Signal(sig); err != nil {
		slog.Warn("Failed to relay signal to upstream", "signal", sig.String(), "error", err)
	}
}

func (s *Service) cache() Cache {
	return NewMemoryCache(s.config.CacheSizeBytes, s.config.MaxCacheItemSizeBytes)
}

func (s *Service) targetUrl() *url.URL {
	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d", s.config.TargetPort))
	return url
}

func (s *Service) setEnvironment() {
	// Set PORT to be inherited by the upstream process.
	os.Setenv("PORT", fmt.Sprintf("%d", s.config.TargetPort))
}
