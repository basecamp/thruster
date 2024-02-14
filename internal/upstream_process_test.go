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

		p := NewUpstreamProcess("sleep", "10")

		go func() {
			exitCode, err = p.Run()
		}()

		<-p.Started
		p.Signal(syscall.SIGTERM)

		assert.NoError(t, err)
		assert.Equal(t, 0, exitCode)
	})
}
