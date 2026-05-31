package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"
)

type serviceTimeouts struct {
	dialTimeout        time.Duration
	portCheckInterval  time.Duration
	fastFailureWait    time.Duration
	gracefulShutdown   time.Duration
	shutdownEscalation time.Duration
	sigkillWait        time.Duration
	finalReapWait      time.Duration
}

type Service struct {
	config   *Config
	dial     dialer
	timeouts serviceTimeouts
}

type upstreamResult struct {
	exitCode int
	err      error
}

// dialer type for injecting net.DialContext in tests
type dialer func(ctx context.Context, network, address string) (net.Conn, error)

func NewService(config *Config) *Service {
	return &Service{
		config: config,
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, address)
		},
		timeouts: serviceTimeouts{
			dialTimeout:        100 * time.Millisecond,
			portCheckInterval:  100 * time.Millisecond,
			fastFailureWait:    10 * time.Millisecond,
			gracefulShutdown:   10 * time.Second,
			shutdownEscalation: 5 * time.Second,
			sigkillWait:        1 * time.Second,
			finalReapWait:      5 * time.Second,
		},
	}
}

func (s *Service) Run() int {
	// Initialize the signal channel early so it can be managed cleanly
	// and we don't miss signals while booting.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, terminationSignals...)
	defer signal.Stop(signalChan)

	if s.config.WaitForTargetPort {
		if s.isPortOpen() {
			slog.Error("Target port is already in use before starting upstream", "port", s.config.TargetPort)
			return 1
		}
	}

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

	resultChan := make(chan upstreamResult, 1)

	// Start the upstream process
	go func() {
		exitCode, err := upstream.Run()
		resultChan <- upstreamResult{exitCode: exitCode, err: err}
	}()

	if s.config.WaitForTargetPort {
		upstreamRes, sig, err := s.waitForTargetPort(resultChan, signalChan, s.dial)
		if err != nil {
			slog.Error("Failed waiting for target port", "error", err)

			// Upstream has already exited, no need to signal
			if upstreamRes != nil {
				slog.Info("Upstream process is already dead.")
				return resolveExitCode(*upstreamRes, 1)
			}

			// Determine which signal to use and calculate the appropriate fallback code
			relaySig := defaultTerminationSignal
			fallbackExitCode := exitCodeFromSignal(defaultTerminationSignal)

			if sig != nil {
				relaySig = sig
				fallbackExitCode = exitCodeFromSignal(sig)
			}

			// Upstream is still running but port never opened (or we were interrupted), shut it down
			return s.terminateUpstream(upstream, resultChan, relaySig, s.timeouts.shutdownEscalation, fallbackExitCode)
		}
		slog.Info("Upstream service is bound to port, starting proxy server.")
	} else {
		// Non-blocking wait to catch fast command failures without penalizing success.
		timer := time.NewTimer(s.timeouts.fastFailureWait)
		select {
		case result := <-resultChan:
			stopTimer(timer)
			slog.Error("Upstream process exited prematurely", "command", s.config.UpstreamCommand, "exit_code", result.exitCode, "error", result.err)
			return resolveExitCode(result, 1)
		case <-timer.C:
			// Upstream is running (or didn't fail instantly), proceed
		}
	}

	if err := server.Start(); err != nil {
		slog.Error("Failed to start proxy server", "error", err)
		return s.terminateUpstream(upstream, resultChan, defaultTerminationSignal, s.timeouts.shutdownEscalation, exitCodeFromSignal(defaultTerminationSignal))
	}
	defer server.Stop()

	return s.awaitTermination(upstream, resultChan, signalChan)
}

// Private

// terminateUpstream encapsulates the escalation policy: signal -> wait -> force kill -> reap
func (s *Service) terminateUpstream(upstream *UpstreamProcess, resultChan <-chan upstreamResult, sig os.Signal, timeout time.Duration, fallbackExitCode int) int {
	if upstream == nil {
		return fallbackExitCode
	}

	// Wait for the upstream process to either start successfully or fail immediately.
	// This prevents signaling before the process is fully initialized.
	select {
	case result := <-resultChan:
		slog.Info("Upstream process terminated before signal could be sent.")
		return resolveExitCode(result, fallbackExitCode)
	case <-upstream.Started():
		// Process is running (or failed to start), proceed
	}

	slog.Info("Sending signal to upstream process...", "signal", sig)
	if err := upstream.Signal(sig); err != nil {
		slog.Error("Failed to send signal to upstream process", "error", err)
	}

	if res, ok := waitResult(resultChan, timeout); ok {
		slog.Info("Upstream process terminated after signal.")
		return resolveExitCode(res, fallbackExitCode)
	}

	slog.Warn("Upstream process did not terminate within timeout, killing it.", "timeout", timeout)

	// Map safely to os.Kill to honor the UpstreamProcess cross-platform encapsulation
	if err := upstream.Signal(os.Kill); err != nil {
		slog.Error("Failed to send KILL signal to upstream process", "error", err)
	}

	if res, ok := waitResult(resultChan, s.timeouts.sigkillWait); ok {
		return resolveExitCode(res, fallbackExitCode)
	}

	// Ensure we do not orphan the running process, but provide a hard upper bound
	slog.Error("Upstream process still running after os.Kill, waiting for OS to reap it...", "timeout", s.timeouts.finalReapWait)
	if res, ok := waitResult(resultChan, s.timeouts.finalReapWait); ok {
		return resolveExitCode(res, fallbackExitCode)
	}

	slog.Error("Upstream process completely unresponsive, exiting wrapper and abandoning child.")
	return fallbackExitCode
}

func (s *Service) isPortOpen() bool {
	portStr := strconv.Itoa(s.config.TargetPort)
	addrs := []string{
		net.JoinHostPort("127.0.0.1", portStr),
		net.JoinHostPort("::1", portStr),
	}

	for _, addr := range addrs {
		dialCtx, cancelDial := context.WithTimeout(context.Background(), s.timeouts.dialTimeout)
		conn, err := s.dial(dialCtx, "tcp", addr)
		cancelDial()

		if err == nil {
			conn.Close() // Port is open
			return true
		}
	}
	return false
}

func (s *Service) waitForTargetPort(resultChan <-chan upstreamResult, signalChan <-chan os.Signal, dial dialer) (*upstreamResult, os.Signal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.WaitForTargetPortTimeout)
	defer cancel()

	portStr := strconv.Itoa(s.config.TargetPort)
	addrs := []string{
		net.JoinHostPort("127.0.0.1", portStr),
		net.JoinHostPort("::1", portStr),
	}
	slog.Info("Waiting for upstream to bind to port", "addresses", addrs)

	tryDial := func() bool {
		for _, addr := range addrs {
			// Cap the dial attempt to prevent unnecessary blocks on individual checks
			dialCtx, cancelDial := context.WithTimeout(ctx, s.timeouts.dialTimeout)
			conn, err := dial(dialCtx, "tcp", addr)
			cancelDial()

			if err == nil {
				conn.Close() // Port is open
				return true
			}
		}
		return false
	}

	prematureExitError := func(result upstreamResult) (*upstreamResult, os.Signal, error) {
		if result.err != nil {
			return &result, nil, fmt.Errorf("upstream process exited prematurely with code %d: %w", result.exitCode, result.err)
		}
		return &result, nil, fmt.Errorf("upstream process exited prematurely with code %d", result.exitCode)
	}

	checkPrematureExit := func() (*upstreamResult, os.Signal, error) {
		select {
		case result := <-resultChan:
			return prematureExitError(result)
		default:
			return nil, nil, nil
		}
	}

	// Attempt a fast TCP connection immediately to prevent unnecessary latency on boot
	if tryDial() {
		return checkPrematureExit()
	}

	// Fallback to checking continuously
	ticker := time.NewTicker(s.timeouts.portCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("timed out after %v waiting for port %d", s.config.WaitForTargetPortTimeout, s.config.TargetPort)

		case sig := <-signalChan:
			return nil, sig, fmt.Errorf("received signal %v while waiting for target port", sig)

		case result := <-resultChan:
			return prematureExitError(result)

		case <-ticker.C:
			if tryDial() {
				return checkPrematureExit()
			}
		}
	}
}

func (s *Service) awaitTermination(upstream *UpstreamProcess, resultChan <-chan upstreamResult, signalChan <-chan os.Signal) int {
	handleResult := func(result upstreamResult) int {
		slog.Info("Wrapped process exited on its own.", "exit_code", result.exitCode)
		if result.err != nil {
			slog.Error("Wrapped process failed", "command", s.config.UpstreamCommand, "args", s.config.UpstreamArgs, "error", result.err)
			// Return the upstream's exit code if available, fallback to 1 otherwise
			return resolveExitCode(result, 1)
		}
		return result.exitCode
	}

	select {
	case result := <-resultChan:
		return handleResult(result)

	case sig := <-signalChan:
		// Prioritize an already-available upstream result to prevent race conditions
		select {
		case result := <-resultChan:
			return handleResult(result)
		default:
		}

		slog.Info("Received signal, shutting down.", "signal", sig)
		fallback := exitCodeFromSignal(sig)
		return s.terminateUpstream(upstream, resultChan, sig, s.timeouts.gracefulShutdown, fallback)
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
	os.Setenv("PORT", strconv.Itoa(s.config.TargetPort))
}

// stopTimer safely stops a timer and drains its channel if it already fired
func stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// waitResult wraps waiting for a process result with a timer, returning whether a result was received
func waitResult(resultChan <-chan upstreamResult, timeout time.Duration) (upstreamResult, bool) {
	timer := time.NewTimer(timeout)
	defer stopTimer(timer)

	select {
	case res := <-resultChan:
		return res, true
	case <-timer.C:
		return upstreamResult{}, false
	}
}

// resolveExitCode returns the upstream's exit code if non-zero, otherwise the provided fallback
func resolveExitCode(result upstreamResult, fallback int) int {
	if result.exitCode != 0 {
		return result.exitCode
	}
	return fallback
}
