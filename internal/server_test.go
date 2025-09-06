package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerDefaultProtocols(t *testing.T) {
	config, err := NewConfig()
	require.NoError(t, err)

	server := NewServer(config, nil)

	s := server.defaultHttpServer(":8080")

	assert.True(t, s.Protocols.HTTP1())
	assert.True(t, s.Protocols.HTTP2())
	assert.False(t, s.Protocols.UnencryptedHTTP2())
}

func TestServerEnabledH2CWhenConfigProvided(t *testing.T) {
	usingEnvVar(t, "H2C_ENABLED", "true")

	config, err := NewConfig()
	require.NoError(t, err)

	server := NewServer(config, nil)

	s := server.defaultHttpServer(":8080")

	assert.True(t, s.Protocols.HTTP1())
	assert.True(t, s.Protocols.HTTP2())
	assert.True(t, s.Protocols.UnencryptedHTTP2())
}
