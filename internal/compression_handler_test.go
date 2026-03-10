package internal

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionHandler(t *testing.T) {
	largeBody := strings.Repeat("A", 2000)

	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, err := w.Write([]byte(largeBody))
		require.NoError(t, err)
	})

	t.Run("compresses responses with gzip", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		reader, err := gzip.NewReader(rr.Body)
		require.NoError(t, err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, largeBody, string(body))
	})

	t.Run("compresses responses with zstd", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "zstd")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "zstd", rr.Header().Get("Content-Encoding"))

		reader, err := zstd.NewReader(rr.Body)
		require.NoError(t, err)
		defer reader.Close()
		body, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, largeBody, string(body))
	})

	t.Run("prefers zstd when client accepts both", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip, zstd")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "zstd", rr.Header().Get("Content-Encoding"))
	})

	t.Run("applies jitter when configured", func(t *testing.T) {
		handler := NewCompressionHandler(32, false, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		require.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Check for GZIP header with FCOMMENT flag (0x10)
		bodyBytes := rr.Body.Bytes()
		require.Greater(t, len(bodyBytes), 10)
		hasComment := (bodyBytes[3] & 0x10) != 0
		assert.True(t, hasComment, "Expected FCOMMENT flag due to jitter")
	})

	t.Run("wraps with guard when disableOnAuth is true", func(t *testing.T) {
		handler := NewCompressionHandler(0, true, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Cookie", "session=secret")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should NOT be compressed due to Cookie header
		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, largeBody, rr.Body.String())
	})

	t.Run("compresses authenticated requests when disableOnAuth is false", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Cookie", "session=secret")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	})
}
