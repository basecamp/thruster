package internal

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_BreachMitigation(t *testing.T) {
	// Mock upstream handler
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(strings.Repeat("A", 2000)))
	})
	upstreamServer := httptest.NewServer(upstream)
	defer upstreamServer.Close()

	upstreamURL, _ := url.Parse(upstreamServer.URL)

	cache := NewMemoryCache(1024, 1024)

	opts := HandlerOptions{
		targetUrl:                    upstreamURL,
		cache:                        cache,
		gzipCompressionEnabled:       true,
		gzipCompressionDisableOnAuth: false,
		gzipCompressionJitter:        32,
	}

	handler := NewHandler(opts)

	t.Run("Public request is compressed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Verify content
		reader, err := gzip.NewReader(rr.Body)
		require.NoError(t, err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("A", 2000), string(body))
	})

	t.Run("Authenticated request IS compressed by default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Cookie", "session=secret")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Verify content
		reader, err := gzip.NewReader(rr.Body)
		require.NoError(t, err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, strings.Repeat("A", 2000), string(body))
	})

	t.Run("Authenticated request is NOT compressed when guard is ENABLED", func(t *testing.T) {
		// Create a handler with the guard enabled
		guardOpts := opts
		guardOpts.gzipCompressionDisableOnAuth = true
		guardHandler := NewHandler(guardOpts)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Cookie", "session=secret")
		rr := httptest.NewRecorder()

		guardHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		// Should NOT be compressed
		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, strings.Repeat("A", 2000), rr.Body.String())
	})

	t.Run("Jitter is applied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		require.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Check for GZIP header with FCOMMENT flag (0x10)
		// Header structure: ID1(1f) ID2(8b) CM(08) FLG
		bodyBytes := rr.Body.Bytes()
		require.Greater(t, len(bodyBytes), 10)
		assert.Equal(t, byte(0x1f), bodyBytes[0])
		assert.Equal(t, byte(0x8b), bodyBytes[1])
		assert.Equal(t, byte(0x08), bodyBytes[2])

		// Check if FCOMMENT flag (bit 4) is set
		// Jitter adds a comment, so this flag should be set
		hasComment := (bodyBytes[3] & 0x10) != 0
		assert.True(t, hasComment, "Expected FCOMMENT flag to be set in GZIP header due to jitter")
	})
}
