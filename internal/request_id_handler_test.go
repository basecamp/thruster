package internal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDHandlerAddsHeaderWhenMissing(t *testing.T) {
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-ID")
	})

	handler := NewRequestIDHandler(true, nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.NotEmpty(t, capturedHeader)
	_, err := uuid.Parse(capturedHeader)
	assert.NoError(t, err)
	assert.Equal(t, capturedHeader, w.Header().Get("X-Request-ID"))
}

func TestRequestIDHandlerPreservesExistingHeaderWhenTrusted(t *testing.T) {
	existingHeader := "id-from-downstream"
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-ID")
	})

	handler := NewRequestIDHandler(true, nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", existingHeader)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, existingHeader, capturedHeader)
	assert.Equal(t, capturedHeader, w.Header().Get("X-Request-ID"))
}

func TestRequestIDHandlerReplacesExistingHeaderWhenNotTrusted(t *testing.T) {
	existingHeader := "id-from-client"
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-ID")
	})

	handler := NewRequestIDHandler(false, nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", existingHeader)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.NotEqual(t, existingHeader, capturedHeader)
	_, err := uuid.Parse(capturedHeader)
	assert.NoError(t, err)
	assert.Equal(t, capturedHeader, w.Header().Get("X-Request-ID"))
}

func TestLoggableRequestIDTruncatesLongValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", strings.Repeat("a", 300))

	assert.Equal(t, strings.Repeat("a", 255), loggableRequestID(r))
}
