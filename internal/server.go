package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
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

func (s *Server) Start() error {
	httpAddress := fmt.Sprintf(":%d", s.config.HttpPort)
	httpsAddress := fmt.Sprintf(":%d", s.config.HttpsPort)

	tlsProvider := s.tlsProvider()
	if tlsProvider != nil {
		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = tlsProvider.HTTPHandler(http.HandlerFunc(httpRedirectHandler))

		s.httpsServer = s.defaultHttpServer(httpsAddress)
		s.httpsServer.TLSConfig = tlsProvider.TLSConfig()
		s.httpsServer.Handler = s.handler

		httpListener, err := net.Listen("tcp", httpAddress)
		if err != nil {
			slog.Error("Failed to start HTTP listener", "error", err)
			return err
		}

		httpsListener, err := net.Listen("tcp", httpsAddress)
		if err != nil {
			slog.Error("Failed to start HTTPS listener", "error", err)
			return err
		}

		go func() { _ = s.httpServer.Serve(httpListener) }()
		go func() { _ = s.httpsServer.ServeTLS(httpsListener, "", "") }()

		slog.Info("Server started", "http", httpAddress, "https", httpsAddress, "tls_domain", s.config.TLSDomains)
		return nil
	} else {
		s.httpsServer = nil
		s.httpServer = s.defaultHttpServer(httpAddress)
		s.httpServer.Handler = s.handler

		httpListener, err := net.Listen("tcp", httpAddress)
		if err != nil {
			slog.Error("Failed to start HTTP listener", "error", err)
			return err
		}

		go func() { _ = s.httpServer.Serve(httpListener) }()

		slog.Info("Server started", "http", httpAddress)
		return nil
	}
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	defer slog.Info("Server stopped")

	slog.Info("Server stopping")

	_ = s.httpServer.Shutdown(ctx)
	if s.httpsServer != nil {
		_ = s.httpsServer.Shutdown(ctx)
	}
}

func (s *Server) tlsProvider() TLSProvider {
	if !s.config.HasTLS() {
		return nil
	}
	if s.config.TLSLocal {
		return NewLocalTLSProvider(s.config.StoragePath)
	}
	return NewAutocertTLSProvider(
		s.config.StoragePath,
		s.config.TLSDomains,
		s.config.ACMEDirectoryURL,
		s.config.EAB_KID,
		s.config.EAB_HMACKey,
	)
}

func (s *Server) defaultHttpServer(addr string) *http.Server {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)

	if s.config.H2CEnabled {
		slog.Debug("Enabling h2c")

		// Enable h2c support
		protocols.SetUnencryptedHTTP2(true)
	}

	return &http.Server{
		Addr:         addr,
		IdleTimeout:  s.config.HttpIdleTimeout,
		ReadTimeout:  s.config.HttpReadTimeout,
		WriteTimeout: s.config.HttpWriteTimeout,
		Protocols:    protocols,
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
