package internal

import (
	"os"
	"os/signal"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamProcess(t *testing.T) {
	t.Run("return exit code on exit", func(t *testing.T) {
		p := NewUpstreamProcess("false")
		exitCode, err := p.Run()

		assert.NoError(t, err)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("returns error if signaled before running", func(t *testing.T) {
		p := NewUpstreamProcess("echo", "hello")

		// Attempt to signal before p.Run() is called
		err := p.Signal(os.Interrupt)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrProcessNotRunning)
	})
}

// TestUpstreamHelperProcess is a hermetic cross-platform target for signal testing
func TestUpstreamHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_UPSTREAM_HELPER_PROCESS") != "1" {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, terminationSignals...)
	defer signal.Stop(c)
	sig := <-c

	os.Exit(exitCodeFromSignal(sig))
}

func TestUpstreamProcess_Signal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support graceful interrupt signaling natively via os.Process")
	}

	t.Run("successfully signals a running process", func(t *testing.T) {
		var exitCode int
		var runErr error
		done := make(chan struct{})

		p := NewUpstreamProcess(os.Args[0], "-test.run=TestUpstreamHelperProcess")
		p.cmd.Env = append(os.Environ(), "GO_WANT_UPSTREAM_HELPER_PROCESS=1")

		t.Cleanup(func() {
			if p.cmd != nil && p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
		})

		go func() {
			exitCode, runErr = p.Run()
			close(done)
		}()

		select {
		case <-p.Started():
			// Process has been spawned
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for upstream to start")
		}

		err := p.Signal(os.Interrupt)
		assert.NoError(t, err)

		select {
		case <-done:
			// Process exited
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for upstream to exit")
		}

		assert.NoError(t, runErr)
		assert.Equal(t, exitCodeFromSignal(os.Interrupt), exitCode)
	})
}
