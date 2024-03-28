package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
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

	if s.config.TLSDomain != "" {
		manager := s.certManager()

		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = manager.HTTPHandler(http.HandlerFunc(httpRedirectHandler))

		s.httpsServer = s.defaultHttpServer(httpsAddress)
		s.httpsServer.TLSConfig = manager.TLSConfig()
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
