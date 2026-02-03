package internal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/idna"
)

type localTLSProvider struct {
	storagePath string
}

func NewLocalTLSProvider(storagePath string) TLSProvider {
	return &localTLSProvider{
		storagePath: storagePath,
	}
}

func (p *localTLSProvider) HTTPHandler(h http.Handler) http.Handler {
	return h
}

func (p *localTLSProvider) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: p.getCertificate,
		NextProtos: []string{
			"h2", "http/1.1", // enable HTTP/2
		},
	}
}

func (p *localTLSProvider) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := hello.ServerName
	if name == "" {
		return nil, errors.New("thruster/local_tls: missing server name")
	}

	name, err := idna.Lookup.ToASCII(name)
	if err != nil {
		return nil, errors.New("thruster/local_tls: server name contains invalid character")
	}

	keyUsage := x509.KeyUsageDigitalSignature
	keyUsage |= x509.KeyUsageKeyEncipherment

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Thruster Local"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 10 * 24 * time.Hour),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(name); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, name)
	}

	authority, err := p.getAuthority()
	if err != nil {
		return nil, err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	authcert, err := x509.ParseCertificate(authority.Certificate[0])
	if err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, authcert, &priv.PublicKey, authority.PrivateKey)
	if err != nil {
		return nil, err
	}

	cert := &tls.Certificate{
		Certificate: [][]byte{authority.Certificate[0], derBytes},
		PrivateKey:  authority.PrivateKey,
	}

	slog.Debug("TLS: issued local certificate for", "name", name)

	return cert, nil
}

func (p *localTLSProvider) getAuthority() (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/authority.crt", p.storagePath), fmt.Sprintf("%s/authority.pem", p.storagePath))
	if err == nil {
		return &cert, nil
	}

	err = os.MkdirAll(p.storagePath, 0750)
	if err != nil {
		return nil, err
	}

	keyUsage := x509.KeyUsageDigitalSignature
	keyUsage |= x509.KeyUsageKeyEncipherment
	keyUsage |= x509.KeyUsageCertSign

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Thruster Local CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 10 * 24 * time.Hour),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certOut, err := os.Create(fmt.Sprintf("%s/authority.crt", p.storagePath))
	if err != nil {
		return nil, err
	}

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, err
	}

	if err := certOut.Close(); err != nil {
		return nil, err
	}

	keyOut, err := os.Create(fmt.Sprintf("%s/authority.pem", p.storagePath))
	if err != nil {
		return nil, err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, err
	}

	if err := keyOut.Close(); err != nil {
		return nil, err
	}

	cer := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	return &cer, nil
}
