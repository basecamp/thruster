package internal

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestStartHandler(t *testing.T) {
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-Start")
	})

	handler := NewRequestStartHandler(nextHandler)

	before := time.Now().UnixMilli()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	after := time.Now().UnixMilli()

	assert.NotEmpty(t, capturedHeader)
	assert.Regexp(t, `^t=\d+$`, capturedHeader)

	timestampStr := capturedHeader[2:]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}

func TestRequestStartHandlerDoesNotOverwriteExistingHeader(t *testing.T) {
	existingHeader := "t=1234567890"
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-Start")
	})

	handler := NewRequestStartHandler(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-Start", existingHeader)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, existingHeader, capturedHeader)
}
