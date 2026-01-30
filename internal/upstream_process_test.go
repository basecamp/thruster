package internal

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpstreamProcess(t *testing.T) {
	t.Run("return exit code on exit", func(t *testing.T) {
		p := NewUpstreamProcess("false")
		exitCode, err := p.Run()

		assert.NoError(t, err)
		assert.Equal(t, 1, exitCode)
	})

	t.Run("signal a process to stop it", func(t *testing.T) {
		var exitCode int
		var err error
		done := make(chan struct{})

		p := NewUpstreamProcess("sleep", "10")

		go func() {
			exitCode, err = p.Run()
			close(done)
		}()

		<-p.Started
		assert.NoError(t, p.Signal(syscall.SIGTERM))
		<-done

		assert.NoError(t, err)
		assert.Equal(t, 128+int(syscall.SIGTERM), exitCode)
	})
}
