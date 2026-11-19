package internal

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Run_PortAlreadyInUse(t *testing.T) {
	service := &Service{
		config: &Config{
			UpstreamCommand:   os.Args[0],
			UpstreamArgs:      []string{"-test.run=TestHelperProcess"},
			TargetPort:        3000,
			WaitForTargetPort: true,
		},
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Simulate port already being open before upstream starts
			client, server := net.Pipe()
			server.Close()
			return client, nil
		},
		timeouts: serviceTimeouts{
			dialTimeout: 10 * time.Millisecond,
		},
	}

	exitCode := service.Run()

	// Should fail immediately because the port is already open
	assert.Equal(t, 1, exitCode)
}

func TestService_waitForTargetPort(t *testing.T) {
	t.Run("success when port opens", func(t *testing.T) {
		service := &Service{
			config: &Config{
				TargetPort:               3000,
				WaitForTargetPortTimeout: 2 * time.Second, // Relaxed margins for CI
			},
			timeouts: serviceTimeouts{
				dialTimeout:       100 * time.Millisecond,
				portCheckInterval: 100 * time.Millisecond,
			},
		}

		resultChan := make(chan upstreamResult, 1)
		signalChan := make(chan os.Signal, 1)

		dialAttempts := 0
		mockDial := func(ctx context.Context, network, address string) (net.Conn, error) {
			dialAttempts++
			// First few iterations fail, then it opens
			if dialAttempts >= 3 {
				client, server := net.Pipe()
				server.Close() // Close the other end to prevent resource leaks
				return client, nil
			}
			return nil, errors.New("connection refused")
		}

		_, _, err := service.waitForTargetPort(resultChan, signalChan, mockDial)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, dialAttempts, 3)
	})

	t.Run("timeout when port never opens", func(t *testing.T) {
		service := &Service{
			config: &Config{
				TargetPort:               3000,
				WaitForTargetPortTimeout: 300 * time.Millisecond, // Slightly longer to prevent flakes
			},
			timeouts: serviceTimeouts{
				dialTimeout:       50 * time.Millisecond,
				portCheckInterval: 50 * time.Millisecond,
			},
		}

		resultChan := make(chan upstreamResult, 1)
		signalChan := make(chan os.Signal, 1)

		mockDial := func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		}

		_, _, err := service.waitForTargetPort(resultChan, signalChan, mockDial)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "timed out after")
	})

	t.Run("returns error if upstream exits early", func(t *testing.T) {
		service := &Service{
			config: &Config{
				TargetPort:               3000,
				WaitForTargetPortTimeout: 2 * time.Second,
			},
			timeouts: serviceTimeouts{
				dialTimeout:       100 * time.Millisecond,
				portCheckInterval: 100 * time.Millisecond,
			},
		}

		resultChan := make(chan upstreamResult, 1)
		signalChan := make(chan os.Signal, 1)

		mockDial := func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		}

		// Simulate the upstream crashing immediately (e.g. bad command)
		resultChan <- upstreamResult{exitCode: 1, err: errors.New("command not found")}

		res, _, err := service.waitForTargetPort(resultChan, signalChan, mockDial)

		require.Error(t, err)
		require.NotNil(t, res)
		assert.Contains(t, err.Error(), "upstream process exited prematurely")
	})

	t.Run("aborts early if signal is received", func(t *testing.T) {
		service := &Service{
			config: &Config{
				TargetPort:               3000,
				WaitForTargetPortTimeout: 5 * time.Second, // Long enough to ensure we don't hit normal timeout
			},
			timeouts: serviceTimeouts{
				dialTimeout:       100 * time.Millisecond,
				portCheckInterval: 100 * time.Millisecond,
			},
		}

		resultChan := make(chan upstreamResult, 1)
		signalChan := make(chan os.Signal, 1)

		mockDial := func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		}

		// Fire a signal shortly after starting to interrupt the wait loop
		go func() {
			time.Sleep(50 * time.Millisecond)
			signalChan <- os.Interrupt
		}()

		_, sig, err := service.waitForTargetPort(resultChan, signalChan, mockDial)

		require.Error(t, err)
		assert.Equal(t, os.Interrupt, sig)
		assert.Contains(t, err.Error(), "received signal interrupt while waiting for target port")
	})
}

func TestService_Run_WaitForTargetPortFailureCleanup(t *testing.T) {
	service := &Service{
		config: &Config{
			UpstreamCommand:          os.Args[0],
			UpstreamArgs:             []string{"-test.run=TestHelperProcess"},
			TargetPort:               3000,
			WaitForTargetPort:        true,
			WaitForTargetPortTimeout: 300 * time.Millisecond,
			CacheSizeBytes:           1024,
			MaxCacheItemSizeBytes:    1024,
		},
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
		timeouts: serviceTimeouts{
			dialTimeout:        100 * time.Millisecond,
			portCheckInterval:  100 * time.Millisecond,
			fastFailureWait:    100 * time.Millisecond,
			gracefulShutdown:   1 * time.Second,
			shutdownEscalation: 1 * time.Second,
			sigkillWait:        1 * time.Second,
			finalReapWait:      1 * time.Second,
		},
	}

	// Start helper in an env that expects to catch an interrupt and gracefully die
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	exitCode := service.Run()

	// The wrapper correctly favors the actual exit code over the fallback.
	// On Unix, defaultTerminationSignal is SIGTERM, so the helper catches it and yields 143.
	// On Windows, it is os.Kill, which cannot be caught, and the OS kills the process (typically yielding 1).
	expectedExitCode := exitCodeFromSignal(defaultTerminationSignal)
	assert.Contains(t, []int{1, expectedExitCode}, exitCode)
}

func TestService_Run_EscalatesToKill(t *testing.T) {
	service := &Service{
		config: &Config{
			UpstreamCommand:          os.Args[0],
			UpstreamArgs:             []string{"-test.run=TestHelperProcess"},
			TargetPort:               3000,
			WaitForTargetPort:        true,
			WaitForTargetPortTimeout: 150 * time.Millisecond,
			CacheSizeBytes:           1024,
			MaxCacheItemSizeBytes:    1024,
		},
		dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return nil, errors.New("connection refused")
		},
		timeouts: serviceTimeouts{
			dialTimeout:        100 * time.Millisecond,
			portCheckInterval:  100 * time.Millisecond,
			fastFailureWait:    100 * time.Millisecond,
			gracefulShutdown:   500 * time.Millisecond,
			shutdownEscalation: 500 * time.Millisecond,
			sigkillWait:        500 * time.Millisecond,
			finalReapWait:      500 * time.Millisecond,
		},
	}

	// Start helper in an env that expects to ignore the first signal
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_WANT_HELPER_PROCESS_IGNORE_SIGNAL", "1")

	exitCode := service.Run()

	// Since we initiated the shutdown and escalated to KILL, the OS forcefully reaps the child.
	// This results in 137 on POSIX, and 1 on Windows.
	assert.Contains(t, []int{1, 137}, exitCode)
}

func TestService_Run_FastFailure_ImmediateExit(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_WANT_HELPER_PROCESS_EXIT_CODE", "42")

	service := &Service{
		config: &Config{
			UpstreamCommand:       os.Args[0],
			UpstreamArgs:          []string{"-test.run=TestHelperProcess"},
			WaitForTargetPort:     false,
			HttpPort:              0, // Random OS port
			TargetPort:            3000,
			CacheSizeBytes:        1024,
			MaxCacheItemSizeBytes: 1024,
		},
		timeouts: serviceTimeouts{
			fastFailureWait: 100 * time.Millisecond,
		},
	}

	exitCode := service.Run()
	assert.Equal(t, 42, exitCode)
}

func TestService_Run_FastFailure_Proceeds(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	// Exits automatically after a delay (post-fastFailureWait) so the test gracefully finishes
	t.Setenv("GO_WANT_HELPER_PROCESS_EXIT_AFTER", "500ms")

	service := &Service{
		config: &Config{
			UpstreamCommand:       os.Args[0],
			UpstreamArgs:          []string{"-test.run=TestHelperProcess"},
			WaitForTargetPort:     false,
			HttpPort:              0, // Random OS port
			TargetPort:            3000,
			CacheSizeBytes:        1024,
			MaxCacheItemSizeBytes: 1024,
		},
		timeouts: serviceTimeouts{
			fastFailureWait:  100 * time.Millisecond,
			gracefulShutdown: 1 * time.Second,
			finalReapWait:    1 * time.Second,
		},
	}

	exitCode := service.Run()

	// Since the upstream successfully bypassed the fastFailureWait check,
	// the proxy server should have started, reaching awaitTermination().
	// Upon the helper exiting automatically, awaitTermination returns the 0 exit code.
	assert.Equal(t, 0, exitCode)
}

func TestService_awaitTermination_NormalExit(t *testing.T) {
	// Provide a dummy config so s.config.UpstreamCommand doesn't panic on error logs
	service := &Service{
		config: &Config{
			UpstreamCommand: "dummy",
			UpstreamArgs:    []string{"-v"},
		},
		timeouts: serviceTimeouts{
			gracefulShutdown: 1 * time.Second,
			finalReapWait:    1 * time.Second,
		},
	}
	resultChan := make(chan upstreamResult, 1)
	signalChan := make(chan os.Signal, 1)

	// Simulate successful graceful exit
	resultChan <- upstreamResult{exitCode: 0, err: nil}
	exitCode := service.awaitTermination(nil, resultChan, signalChan)
	assert.Equal(t, 0, exitCode)

	// Simulate error exit
	resultChan <- upstreamResult{exitCode: 1, err: errors.New("crash")}
	exitCode = service.awaitTermination(nil, resultChan, signalChan)
	assert.Equal(t, 1, exitCode)
}

// TestHelperProcess is used to run a hermetic sub-process that correctly
// handles OS-level semantics in a portable way.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if codeStr := os.Getenv("GO_WANT_HELPER_PROCESS_EXIT_CODE"); codeStr != "" {
		if code, err := strconv.Atoi(codeStr); err == nil {
			os.Exit(code)
		}
	}

	if delayStr := os.Getenv("GO_WANT_HELPER_PROCESS_EXIT_AFTER"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			time.Sleep(delay)
			os.Exit(0)
		}
	}

	if os.Getenv("GO_WANT_HELPER_PROCESS_IGNORE_SIGNAL") == "1" {
		c := make(chan os.Signal, 1)
		signal.Notify(c, terminationSignals...)
		defer signal.Stop(c)
		<-c
		// Ignore the signal and wait to be killed forcefully (but bounded)
		time.Sleep(15 * time.Second)
		os.Exit(0)
	}

	// Block until signal is received
	c := make(chan os.Signal, 1)
	signal.Notify(c, terminationSignals...)
	defer signal.Stop(c)
	sig := <-c

	// Exit gracefully using cross-platform signal exit map
	os.Exit(exitCodeFromSignal(sig))
}

func TestService_awaitTermination_Signal(t *testing.T) {
	service := &Service{
		config: &Config{
			UpstreamCommand: os.Args[0],
			UpstreamArgs:    []string{"-test.run=TestHelperProcess"},
		},
		timeouts: serviceTimeouts{
			gracefulShutdown: 5 * time.Second,
			finalReapWait:    1 * time.Second,
		},
	}

	// Run the test suite itself as the hermetic target process
	upstream := NewUpstreamProcess(os.Args[0], "-test.run=TestHelperProcess")
	upstream.cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

	resultChan := make(chan upstreamResult, 1)
	signalChan := make(chan os.Signal, 1)

	// Start the mocked upstream process
	go func() {
		exitCode, err := upstream.Run()
		resultChan <- upstreamResult{exitCode: exitCode, err: err}
	}()
	<-upstream.Started() // Ensure it is ready to receive signals

	// Mock injecting an OS signal directly into the injected channel
	// We relay unmodified, so sending os.Interrupt will hit the helper which catches it
	signalChan <- os.Interrupt

	exitCode := service.awaitTermination(upstream, resultChan, signalChan)

	// Ensure the signal relay returned the correct exit code observed from the upstream
	expectedExitCode := exitCodeFromSignal(os.Interrupt)
	assert.Equal(t, expectedExitCode, exitCode)
}
