package internal

import (
	"fmt"
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
		badGatewayPage:           s.config.BadGatewayPage,
		imageProxyEnabled:        s.config.ImageProxyEnabled,
	}

	handler := NewHandler(handlerOptions)
	server := NewServer(s.config, handler)
	upstream := NewUpstreamProcess(s.config.UpstreamCommand, s.config.UpstreamArgs...)

	server.Start()
	defer server.Stop()

	s.setEnvironment()

	exitCode, err := upstream.Run()
	if err != nil {
		panic(err)
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

	// Set IMAGE_PROXY_PATH, if enabled
	if s.config.ImageProxyEnabled {
		os.Setenv("IMAGE_PROXY_PATH", imageProxyHandlerPath)
	}
}
