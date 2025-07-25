package internal

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRequestStartMiddleware(t *testing.T) {
	var capturedHeader string
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Request-Start")
	})

	middleware := NewRequestStartMiddleware(nextHandler)

	before := time.Now().UnixMilli()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	after := time.Now().UnixMilli()

	assert.NotEmpty(t, capturedHeader)
	assert.Regexp(t, `^t=\d+$`, capturedHeader)

	timestampStr := capturedHeader[2:]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, timestamp, before)
	assert.LessOrEqual(t, timestamp, after)
}