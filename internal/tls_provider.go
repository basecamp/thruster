package internal

import (
	"crypto/tls"
	"net/http"
)

type TLSProvider interface {
	HTTPHandler(h http.Handler) http.Handler
	TLSConfig() *tls.Config
}
