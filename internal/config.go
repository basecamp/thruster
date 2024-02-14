package internal

import (
	"errors"
	"os"
	"strconv"
	"time"
)

const (
	KB = 1024
	MB = 1024 * KB

	defaultTargetPort = 3000

	defaultCacheSize             = 64 * MB
	defaultMaxCacheItemSizeBytes = 1 * MB
	defaultMaxRequestBody        = 0

	defaultStoragePath    = "./storage/thruster"
	defaultBadGatewayPage = "./public/502.html"

	defaultHttpPort         = 80
	defaultHttpsPort        = 443
	defaultHttpIdleTimeout  = 30 * time.Second
	defaultHttpReadTimeout  = 10 * time.Second
	defaultHttpWriteTimeout = 10 * time.Second
)

type Config struct {
	TargetPort      int
	UpstreamCommand string
	UpstreamArgs    []string

	CacheSizeBytes        int
	MaxCacheItemSizeBytes int
	XSendfileEnabled      bool
	MaxRequestBody        int

	SSLDomain      string
	StoragePath    string
	BadGatewayPage string

	HttpPort         int
	HttpsPort        int
	HttpIdleTimeout  time.Duration
	HttpReadTimeout  time.Duration
	HttpWriteTimeout time.Duration
}

func NewConfig() (*Config, error) {
	if len(os.Args) < 2 {
		return nil, errors.New("missing upstream command")
	}

	return &Config{
		TargetPort:      getEnvInt("TARGET_PORT", defaultTargetPort),
		UpstreamCommand: os.Args[1],
		UpstreamArgs:    os.Args[2:],

		CacheSizeBytes:        getEnvInt("CACHE_SIZE", defaultCacheSize),
		MaxCacheItemSizeBytes: getEnvInt("MAX_CACHE_ITEM_SIZE", defaultMaxCacheItemSizeBytes),
		XSendfileEnabled:      getEnvBool("X_SENDFILE_ENABLED", true),
		MaxRequestBody:        getEnvInt("MAX_REQUEST_BODY", defaultMaxRequestBody),

		SSLDomain:      os.Getenv("SSL_DOMAIN"),
		StoragePath:    getEnvString("STORAGE_PATH", defaultStoragePath),
		BadGatewayPage: getEnvString("BAD_GATEWAY_PAGE", defaultBadGatewayPage),

		HttpPort:         getEnvInt("HTTP_PORT", defaultHttpPort),
		HttpsPort:        getEnvInt("HTTPS_PORT", defaultHttpsPort),
		HttpIdleTimeout:  getEnvDuration("HTTP_IDLE_TIMEOUT", defaultHttpIdleTimeout),
		HttpReadTimeout:  getEnvDuration("HTTP_READ_TIMEOUT", defaultHttpReadTimeout),
		HttpWriteTimeout: getEnvDuration("HTTP_WRITE_TIMEOUT", defaultHttpWriteTimeout),
	}, nil
}

func getEnvString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return time.Duration(intValue) * time.Second
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return boolValue
}
