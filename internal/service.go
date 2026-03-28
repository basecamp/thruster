package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Service struct {
	config *Config
}

// Represents the result of the upstream process execution.
type upstreamResult struct {
	exitCode int
	err      error
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

	s.setEnvironment()

	// Channel to receive the result from the upstream process goroutine.
	resultChan := make(chan upstreamResult, 1)

	// Run the upstream process in a separate goroutine
	// This allows us to perform health checks while it starts up
	go func() {
		exitCode, err := upstream.Run()
		resultChan <- upstreamResult{exitCode: exitCode, err: err}
	}()

	// If a health check path is configured, wait for the upstream to become healthy
	if s.config.HttpHealthPath != "" {
		if err := s.performHealthCheck(resultChan); err != nil {
			slog.Error("Upstream health check failed", "error", err)
			// At this point, the upstream process is running but unhealthy
			if err := upstream.Signal(syscall.SIGTERM); err != nil {
				slog.Error("Failed to send signal to upstream process", "error", err)
			}
			return 1
		}
		slog.Info("Upstream service is healthy, starting proxy server.")
	}

	// Now that the upstream is ready, start the main proxy server
	if err := server.Start(); err != nil {
		return 1
	}
	defer server.Stop()

	// Delegate the waiting and signal handling to the new function
	return s.awaitTermination(upstream, resultChan)
}

// Private

func (s *Service) awaitTermination(upstream *UpstreamProcess, resultChan <-chan upstreamResult) int {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case result := <-resultChan:
		// The upstream process finished on its own.
		slog.Info("Wrapped process exited on its own.", "exit_code", result.exitCode)
		if result.err != nil {
			slog.Error("Wrapped process failed", "command", s.config.UpstreamCommand, "args", s.config.UpstreamArgs, "error", result.err)
			return 1
		}
		return result.exitCode

	case sig := <-signalChan:
		// An OS signal was caught
		slog.Info("Received signal, shutting down.", "signal", sig.String())

		// Relay the signal to the child process to allow for graceful shutdown.
		slog.Info("Relaying signal to upstream process...")
		if err := upstream.Signal(sig); err != nil {
			slog.Error("Failed to send signal to upstream process", "error", err)
		}

		// Give the upstream process a moment to shut down gracefully
		// before the defer server.Stop() forcefully cleans up.
		select {
		case <-resultChan:
			slog.Info("Upstream process terminated gracefully after signal.")
		case <-time.After(10 * time.Second):
			slog.Warn("Upstream process did not terminate within 10 seconds of signal.")
		}

		// Exit with a non-zero status code to indicate termination by signal.
		return 1
	}
}

// performHealthCheck polls the health check endpoint until it gets a 200 OK
func (s *Service) performHealthCheck(resultChan <-chan upstreamResult) error {
	// Create a context with a 2-minute timeout (default) for the entire health check process
	ctx, cancel := context.WithTimeout(context.Background(), s.config.HttpHealthDeadline)
	defer cancel()

	// We assume the upstream server binds to the target URL's host
	healthCheckURL := fmt.Sprintf("http://%s:%d%s", s.config.HttpHealthHost, s.config.TargetPort, s.config.HttpHealthPath)
	slog.Info("Starting health checks", "url", healthCheckURL)

	// Use a ticker to check every second (default)
	ticker := time.NewTicker(s.config.HttpHealthInterval)
	defer ticker.Stop()

	// Create an HTTP client with a short timeout for individual requests
	client := &http.Client{
		Timeout: s.config.HttpHealthTimeout,
	}

	for {
		select {
		case <-ctx.Done():
			// Deadline exceeded
			return fmt.Errorf("health check timed out after %v", s.config.HttpHealthDeadline)

		case result := <-resultChan:
			// The upstream process exited before it became healthy
			return fmt.Errorf("upstream process exited prematurely with code %d: %w", result.exitCode, result.err)

		case <-ticker.C:
			// Ticker fired, time to perform a check
			req, err := http.NewRequestWithContext(ctx, "GET", healthCheckURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create health check request: %w", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				// This is expected while the server is starting up (e.g., "connection refused")
				slog.Debug("Health check attempt failed, retrying...", "error", err)
				continue
			}

			// Don't forget to close the body
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				// Success!
				return nil
			}

			slog.Debug("Health check received non-200 status", "status_code", resp.StatusCode)
		}
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
