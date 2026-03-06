package internal

import (
	"crypto/tls"
	"encoding/base64"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

type AutocertTLSProvider struct {
	manager *autocert.Manager
}

func NewAutocertTLSProvider(storagePath string, domains []string, acmeDirectoryURL string, eabKID string, eabHMACKey string) TLSProvider {
	client := &acme.Client{DirectoryURL: acmeDirectoryURL}
	binding := createExternalAccountBinding(eabKID, eabHMACKey)

	slog.Debug("TLS: initializing autocert", "directory", client.DirectoryURL, "using_eab", binding != nil)

	manager := &autocert.Manager{
		Cache:                  autocert.DirCache(storagePath),
		Client:                 client,
		ExternalAccountBinding: binding,
		HostPolicy:             autocert.HostWhitelist(domains...),
		Prompt:                 autocert.AcceptTOS,
	}

	return &AutocertTLSProvider{
		manager: manager,
	}
}

func (p *AutocertTLSProvider) HTTPHandler(h http.Handler) http.Handler {
	return p.manager.HTTPHandler(h)
}

func (p *AutocertTLSProvider) TLSConfig() *tls.Config {
	return p.manager.TLSConfig()
}

func createExternalAccountBinding(kid string, hmacKey string) *acme.ExternalAccountBinding {
	if kid == "" || hmacKey == "" {
		return nil
	}

	key, err := base64.RawURLEncoding.DecodeString(hmacKey)
	if err != nil {
		slog.Error("Error decoding EAB_HMACKey", "error", err)
		return nil
	}

	return &acme.ExternalAccountBinding{
		KID: kid,
		Key: key,
	}
}
