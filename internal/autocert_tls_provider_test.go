package internal

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAutocertTLSProvider(t *testing.T) {
	tmpDir := t.TempDir()
	domains := []string{"example.com", "www.example.com"}
	acmeURL := "https://acme-staging-v02.api.letsencrypt.org/directory"

	provider := NewAutocertTLSProvider(tmpDir, domains, acmeURL, "", "")

	require.NotNil(t, provider)
}

func TestNewAutocertTLSProvider_WithEAB(t *testing.T) {
	tmpDir := t.TempDir()
	domains := []string{"example.com"}
	acmeURL := "https://acme.zerossl.com/v2/DV90"
	eabKID := "test-kid"
	eabHMACKey := "dGVzdC1obWFjLWtleQ" // base64 encoded "test-hmac-key"

	provider := NewAutocertTLSProvider(tmpDir, domains, acmeURL, eabKID, eabHMACKey)

	require.NotNil(t, provider)
}

func TestAutocertTLSProvider_HTTPHandler(t *testing.T) {
	tmpDir := t.TempDir()
	domains := []string{"example.com"}
	acmeURL := "https://acme-staging-v02.api.letsencrypt.org/directory"
	provider := NewAutocertTLSProvider(tmpDir, domains, acmeURL, "", "")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("test"))
		require.NoError(t, err)
	})

	wrapped := provider.HTTPHandler(handler)
	require.NotNil(t, wrapped)
}

func TestAutocertTLSProvider_TLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	domains := []string{"example.com", "www.example.com"}
	acmeURL := "https://acme-staging-v02.api.letsencrypt.org/directory"
	provider := NewAutocertTLSProvider(tmpDir, domains, acmeURL, "", "")

	config := provider.TLSConfig()

	require.NotNil(t, config)
	assert.NotNil(t, config.GetCertificate)
	assert.Contains(t, config.NextProtos, "h2")
	assert.Contains(t, config.NextProtos, "http/1.1")
	assert.Contains(t, config.NextProtos, "acme-tls/1")
}

func TestCreateExternalAccountBinding_ValidBase64(t *testing.T) {
	kid := "test-kid"
	hmacKey := "dGVzdC1obWFjLWtleQ" // base64 encoded "test-hmac-key"

	binding := createExternalAccountBinding(kid, hmacKey)

	require.NotNil(t, binding)
	assert.Equal(t, kid, binding.KID)
	assert.Equal(t, []byte("test-hmac-key"), binding.Key)
}

func TestCreateExternalAccountBinding_InvalidBase64(t *testing.T) {
	kid := "test-kid"
	hmacKey := "not-valid-base64!!!" // Invalid base64

	binding := createExternalAccountBinding(kid, hmacKey)

	// Should return nil on invalid base64
	assert.Nil(t, binding)
}

func TestCreateExternalAccountBinding_EmptyInputs(t *testing.T) {
	// Both empty
	binding := createExternalAccountBinding("", "")
	assert.Nil(t, binding)

	// Only KID
	binding = createExternalAccountBinding("test-kid", "")
	assert.Nil(t, binding)

	// Only HMAC key
	binding = createExternalAccountBinding("", "dGVzdC1obWFjLWtleQ")
	assert.Nil(t, binding)
}
