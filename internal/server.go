package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
  "os"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"crypto/x509"
  "crypto/tls"
  "crypto/rsa"
  "errors"
  "golang.org/x/net/idna"
  "crypto/rand"
  "math/big"
  "crypto/x509/pkix"
  "encoding/pem"
)

type Server struct {
	config      *Config
	handler     http.Handler
	httpServer  *http.Server
	httpsServer *http.Server
}

func NewServer(config *Config, handler http.Handler) *Server {
	return &Server{
		handler: handler,
		config:  config,
	}
}

func (s *Server) Start() {
	httpAddress := fmt.Sprintf(":%d", s.config.HttpPort)
	httpsAddress := fmt.Sprintf(":%d", s.config.HttpsPort)

	if s.config.TLSDomain != "" && s.config.TLSLocal == false {
		manager := s.certManager()

		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = manager.HTTPHandler(http.HandlerFunc(httpRedirectHandler))

		s.httpsServer = s.defaultHttpServer(httpsAddress)
		s.httpsServer.TLSConfig = manager.TLSConfig()
		s.httpsServer.Handler = s.handler

		go s.httpServer.ListenAndServe()
		go s.httpsServer.ListenAndServeTLS("", "")

		slog.Info("Server started", "http", httpAddress, "https", httpsAddress, "tls_domain", s.config.TLSDomain)
  } else if s.config.TLSDomain != "" && s.config.TLSLocal {
		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = http.HandlerFunc(httpRedirectHandler)

		s.httpsServer = s.defaultHttpServer(httpsAddress)
		s.httpsServer.TLSConfig = s.localTLSConfig()
		s.httpsServer.Handler = s.handler

		go s.httpServer.ListenAndServe()
		go s.httpsServer.ListenAndServeTLS("", "")

		slog.Info("Server started", "http", httpAddress, "https", httpsAddress, "tls_domain", s.config.TLSDomain)
	} else {
		s.httpsServer = nil
		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = s.handler

		go s.httpServer.ListenAndServe()

		slog.Info("Server started", "http", httpAddress)
	}
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	defer slog.Info("Server stopped")

	slog.Info("Server stopping")

	s.httpServer.Shutdown(ctx)
	if s.httpsServer != nil {
		s.httpsServer.Shutdown(ctx)
	}
}

func (s *Server) certManager() *autocert.Manager {
	client := &acme.Client{DirectoryURL: s.config.ACMEDirectoryURL}
	binding := s.externalAccountBinding()

	slog.Debug("TLS: initializing", "directory", client.DirectoryURL, "using_eab", binding != nil)

	return &autocert.Manager{
		Cache:                  autocert.DirCache(s.config.StoragePath),
		Client:                 client,
		ExternalAccountBinding: binding,
		HostPolicy:             autocert.HostWhitelist(s.config.TLSDomain),
		Prompt:                 autocert.AcceptTOS,
	}
}

func (s *Server) localTLSConfig() *tls.Config {
  return &tls.Config{
		GetCertificate: s.getLocalCertificate,
		NextProtos: []string{
			"h2", "http/1.1", // enable HTTP/2
		},
	}
}

func (s *Server) getLocalCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
  name := s.config.TLS_DOMAIN
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
    NotBefore: time.Now(),
    NotAfter:  time.Now().Add(365*10*24*time.Hour),
    KeyUsage:  keyUsage,
    ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    BasicConstraintsValid: true,
  }
  
  if ip := net.ParseIP(name); ip != nil {
    template.IPAddresses = append(template.IPAddresses, ip)
  } else {
    template.DNSNames = append(template.DNSNames, name)
  }

  authority, err := s.getLocalAuthority()
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
    Certificate: [][]byte{ authority.Certificate[0], derBytes },
    PrivateKey: authority.PrivateKey,
  } 

  return cert, nil
}

func (s *Server) getLocalAuthority() (*tls.Certificate, error) {

  cert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/authority.crt", s.config.StoragePath), fmt.Sprintf("%s/authority.pem", s.config.StoragePath))
  if err == nil {
    return &cert, nil
  }

  err = os.Mkdir(s.config.StoragePath, 0750)

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
    NotBefore: time.Now(),
    NotAfter:  time.Now().Add(365*10*24*time.Hour),
    KeyUsage:  keyUsage,
    ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    BasicConstraintsValid: true,
    IsCA: true,
  }
  
  priv, err := rsa.GenerateKey(rand.Reader, 2048)
  if err != nil {
		return nil, err
	} 

  derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
  if err != nil {
		return nil, err
	}

  certOut, err := os.Create(fmt.Sprintf("%s/authority.crt", s.config.StoragePath))
	if err != nil {
    return nil, err
	}

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
    return nil, err
	}

	if err := certOut.Close(); err != nil {
	  return nil, err
  }

	keyOut, err := os.Create(fmt.Sprintf("%s/authority.pem", s.config.StoragePath))
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
    Certificate: [][]byte{ derBytes },
    PrivateKey: priv,
  } 

  return &cer, nil
}

func (s *Server) externalAccountBinding() *acme.ExternalAccountBinding {
	if s.config.EAB_KID == "" || s.config.EAB_HMACKey == "" {
		return nil
	}

	key, err := base64.RawURLEncoding.DecodeString(s.config.EAB_HMACKey)
	if err != nil {
		slog.Error("Error decoding EAB_HMACKey", "error", err)
		return nil
	}

	return &acme.ExternalAccountBinding{
		KID: s.config.EAB_KID,
		Key: key,
	}
}

func (s *Server) defaultHttpServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		IdleTimeout:  s.config.HttpIdleTimeout,
		ReadTimeout:  s.config.HttpReadTimeout,
		WriteTimeout: s.config.HttpWriteTimeout,
	}
}

func httpRedirectHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "close")

	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	url := "https://" + host + r.URL.RequestURI()
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}
