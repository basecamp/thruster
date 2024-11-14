package internal

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_tls(t *testing.T) {
	t.Run("with no ENV", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{}, c.TLSDomains)
		assert.False(t, c.HasTLS())
		assert.True(t, c.ForwardHeaders)
	})

	t.Run("with an empty TLS_DOMAIN", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "TLS_DOMAIN", "")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{}, c.TLSDomains)
		assert.False(t, c.HasTLS())
		assert.True(t, c.ForwardHeaders)
	})

	t.Run("with single TLS_DOMAIN", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "TLS_DOMAIN", "example.com")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{"example.com"}, c.TLSDomains)
		assert.True(t, c.HasTLS())
		assert.False(t, c.ForwardHeaders)
	})

	t.Run("with multiple TLS_DOMAIN", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "TLS_DOMAIN", "example.com, example.io")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{"example.com", "example.io"}, c.TLSDomains)
		assert.True(t, c.HasTLS())
		assert.False(t, c.ForwardHeaders)
	})

	t.Run("with TLS_DOMAIN containing whitespace", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "TLS_DOMAIN", "  ,   example.com, example.io,")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{"example.com", "example.io"}, c.TLSDomains)
		assert.True(t, c.HasTLS())
		assert.False(t, c.ForwardHeaders)
	})

	t.Run("overriding with FORWARD_HEADERS when using TLS", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "TLS_DOMAIN", "example.com")
		usingEnvVar(t, "FORWARD_HEADERS", "true")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Equal(t, []string{"example.com"}, c.TLSDomains)
		assert.True(t, c.HasTLS())
		assert.True(t, c.ForwardHeaders)
	})

	t.Run("overriding with FORWARD_HEADERS when not using TLS", func(t *testing.T) {
		usingProgramArgs(t, "thruster", "echo", "hello")
		usingEnvVar(t, "FORWARD_HEADERS", "false")

		c, err := NewConfig()
		require.NoError(t, err)

		assert.Empty(t, c.TLSDomains)
		assert.False(t, c.HasTLS())
		assert.False(t, c.ForwardHeaders)
	})
}

func TestConfig_defaults(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 3000, c.TargetPort)
	assert.Equal(t, "echo", c.UpstreamCommand)
	assert.Equal(t, defaultCacheSize, c.CacheSizeBytes)
	assert.Equal(t, slog.LevelInfo, c.LogLevel)
}

func TestConfig_override_defaults_with_env_vars(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")
	usingEnvVar(t, "TARGET_PORT", "4000")
	usingEnvVar(t, "CACHE_SIZE", "256")
	usingEnvVar(t, "HTTP_READ_TIMEOUT", "5")
	usingEnvVar(t, "X_SENDFILE_ENABLED", "0")
	usingEnvVar(t, "DEBUG", "1")
	usingEnvVar(t, "ACME_DIRECTORY", "https://acme-staging-v02.api.letsencrypt.org/directory")
	usingEnvVar(t, "LOG_REQUESTS", "false")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 4000, c.TargetPort)
	assert.Equal(t, 256, c.CacheSizeBytes)
	assert.Equal(t, 5*time.Second, c.HttpReadTimeout)
	assert.Equal(t, false, c.XSendfileEnabled)
	assert.Equal(t, slog.LevelDebug, c.LogLevel)
	assert.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", c.ACMEDirectoryURL)
	assert.Equal(t, false, c.LogRequests)
}

func TestConfig_override_defaults_with_env_vars_using_prefix(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")
	usingEnvVar(t, "THRUSTER_TARGET_PORT", "4000")
	usingEnvVar(t, "THRUSTER_CACHE_SIZE", "256")
	usingEnvVar(t, "THRUSTER_HTTP_READ_TIMEOUT", "5")
	usingEnvVar(t, "THRUSTER_X_SENDFILE_ENABLED", "0")
	usingEnvVar(t, "THRUSTER_DEBUG", "1")
	usingEnvVar(t, "THRUSTER_LOG_REQUESTS", "0")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 4000, c.TargetPort)
	assert.Equal(t, 256, c.CacheSizeBytes)
	assert.Equal(t, 5*time.Second, c.HttpReadTimeout)
	assert.Equal(t, false, c.XSendfileEnabled)
	assert.Equal(t, slog.LevelDebug, c.LogLevel)
	assert.Equal(t, false, c.LogRequests)
}

func TestConfig_prefixed_variables_take_precedence_over_non_prefixed(t *testing.T) {
	usingProgramArgs(t, "thruster", "echo", "hello")
	usingEnvVar(t, "TARGET_PORT", "3000")
	usingEnvVar(t, "THRUSTER_TARGET_PORT", "4000")

	c, err := NewConfig()
	require.NoError(t, err)

	assert.Equal(t, 4000, c.TargetPort)
}

func TestConfig_return_error_when_no_upstream_command(t *testing.T) {
	usingProgramArgs(t, "thruster")

	_, err := NewConfig()
	require.Error(t, err)
}
