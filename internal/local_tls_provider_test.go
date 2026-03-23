package internal

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalTLSProvider_HTTPHandler(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("test response"))
		require.NoError(t, err)
	})

	wrapped := provider.HTTPHandler(handler)

	// HTTPHandler should just pass through without modification for local TLS
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
}

func TestLocalTLSProvider_TLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	config := provider.TLSConfig()

	require.NotNil(t, config)
	assert.NotNil(t, config.GetCertificate)
	assert.Contains(t, config.NextProtos, "h2")
	assert.Contains(t, config.NextProtos, "http/1.1")
}

func TestLocalTLSProvider_WithDomainName(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "example.com",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	require.NoError(t, err)
	require.NotNil(t, cert)
	assert.NotNil(t, cert.PrivateKey)
	assert.NotEmpty(t, cert.Certificate)

	// Verify the certificate has the correct DNS name
	x509Cert, err := x509.ParseCertificate(cert.Certificate[1])
	require.NoError(t, err)
	assert.Contains(t, x509Cert.DNSNames, "example.com")
	assert.Equal(t, "Thruster Local", x509Cert.Subject.Organization[0])
}

func TestLocalTLSProvider_WithIPAddress(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "127.0.0.1",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify the certificate has the correct IP address
	x509Cert, err := x509.ParseCertificate(cert.Certificate[1])
	require.NoError(t, err)
	assert.Len(t, x509Cert.IPAddresses, 1)
	assert.Equal(t, "127.0.0.1", x509Cert.IPAddresses[0].String())
}

func TestLocalTLSProvider_MissingServerName(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "missing server name")
}

func TestLocalTLSProvider_InvalidServerName(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "invalid\x00name",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	assert.Error(t, err)
	assert.Nil(t, cert)
}

func TestLocalTLSProvider_CreatesNewCA(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "example.com",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify CA files were created
	assert.FileExists(t, filepath.Join(tmpDir, "authority.crt"))
	assert.FileExists(t, filepath.Join(tmpDir, "authority.pem"))

	// Verify the CA certificate properties
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)
	require.NotNil(t, x509Cert)

	require.NoError(t, err)
	assert.True(t, x509Cert.IsCA)
	assert.Equal(t, "Thruster Local CA", x509Cert.Subject.Organization[0])
}

func TestLocalTLSProvider_ReuseCA(t *testing.T) {
	tmpDir := t.TempDir()

	hello := &tls.ClientHelloInfo{
		ServerName: "example.com",
	}

	// This should create a new CA
	provider1 := NewLocalTLSProvider(tmpDir)
	config1 := provider1.TLSConfig()
	cert1, err := config1.GetCertificate(hello)
	require.NoError(t, err)
	require.NotNil(t, cert1)

	// This should reuse the previous CA
	provider2 := NewLocalTLSProvider(tmpDir)
	config2 := provider2.TLSConfig()
	cert2, err := config2.GetCertificate(hello)
	require.NoError(t, err)
	require.NotNil(t, cert2)

	// Should be the same certificate (index 0 is the CA)
	assert.Equal(t, cert1.Certificate[0], cert2.Certificate[0])
}

func TestLocalTLSProvider_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	// Create a test server with the TLS config
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("Hello, TLS!"))
		require.NoError(t, err)
	})

	server := httptest.NewUnstartedServer(handler)
	server.TLS = provider.TLSConfig()
	server.StartTLS()
	defer server.Close()

	// Create a client that accepts our self-signed certificate
	client := server.Client()

	// Make a request
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLocalTLSProvider_CreatesStorageDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "nested", "storage", "path")
	provider := NewLocalTLSProvider(storageDir)

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(&tls.ClientHelloInfo{ServerName: "example.com"})

	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify the nested directory was created
	info, err := os.Stat(storageDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestLocalTLSProvider_WithInternationalDomain(t *testing.T) {
	tmpDir := t.TempDir()
	provider := NewLocalTLSProvider(tmpDir)

	hello := &tls.ClientHelloInfo{
		ServerName: "zürich.example.com",
	}

	config := provider.TLSConfig()
	cert, err := config.GetCertificate(hello)

	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify the certificate has the punycode-encoded domain
	x509Cert, err := x509.ParseCertificate(cert.Certificate[1])
	require.NoError(t, err)
	assert.Contains(t, x509Cert.DNSNames, "xn--zrich-kva.example.com")
}
