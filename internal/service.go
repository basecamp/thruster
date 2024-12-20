package internal

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
)

type Service struct {
	config *Config
}

func NewService(config *Config) *Service {
	return &Service{
		config: config,
	}
}

func (s *Service) Run() int {
	handlerOptions := HandlerOptions{
		cache:                    s.cache(),
		targetUrl:                s.targetUrl(),
		xSendfileEnabled:         s.config.XSendfileEnabled,
		maxCacheableResponseBody: s.config.MaxCacheItemSizeBytes,
		maxRequestBody:           s.config.MaxRequestBody,
		badGatewayPage:           s.config.BadGatewayPage,
		forwardHeaders:           s.config.ForwardHeaders,
		logRequests:              s.config.LogRequests,
	}

	handler := NewHandler(handlerOptions)
	server := NewServer(s.config, handler)
	upstream := NewUpstreamProcess(s.config.UpstreamCommand, s.config.UpstreamArgs...)

	server.Start()
	defer server.Stop()

	s.setEnvironment()

	exitCode, err := upstream.Run()
	if err != nil {
		slog.Error("Failed to start wrapped process", "command", s.config.UpstreamCommand, "args", s.config.UpstreamArgs, "error", err)
		return 1
	}

	return exitCode
}

// Private

func (s *Service) cache() Cache {
	return NewMemoryCache(s.config.CacheSizeBytes, s.config.MaxCacheItemSizeBytes)
}

func (s *Service) targetUrl() *url.URL {
	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d", s.config.TargetPort))
	return url
}

func (s *Service) setEnvironment() {
	// Set PORT to be inherited by the upstream process.
	os.Setenv("PORT", fmt.Sprintf("%d", s.config.TargetPort))
}
