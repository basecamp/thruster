package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_defaults(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 3000, c.TargetPort)
	assert.Equal(t, "echo", c.UpstreamCommand)
	assert.Equal(t, defaultCacheSize, c.CacheSizeBytes)
}

func TestConfig_override_defaults_with_env_vars(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")
	usingEnvVar(t, "TARGET_PORT", "4000")
	usingEnvVar(t, "CACHE_SIZE", "256")
	usingEnvVar(t, "HTTP_READ_TIMEOUT", "5")
	usingEnvVar(t, "X_SENDFILE_ENABLED", "0")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 4000, c.TargetPort)
	assert.Equal(t, 256, c.CacheSizeBytes)
	assert.Equal(t, 5*time.Second, c.HttpReadTimeout)
	assert.Equal(t, false, c.XSendfileEnabled)
}

func TestConfig_return_error_when_no_upstream_command(t *testing.T) {
	usingProgramArgs(t, "thruster")

	_, err := NewConfig()
	require.Error(t, err)
}
