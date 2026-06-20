package internal

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	t.Run("compresses responses", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, nil, upstream)

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

	t.Run("applies jitter when configured", func(t *testing.T) {
		handler := NewCompressionHandler(32, false, nil, upstream)

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
		handler := NewCompressionHandler(0, true, nil, upstream)

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
		handler := NewCompressionHandler(0, false, nil, upstream)

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.Set("Cookie", "session=secret")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	})
}

func TestCompressionHandler_exceptContentTypes(t *testing.T) {
	largeBody := strings.Repeat("A", 2000)

	upstreamWithType := func(contentType string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, err := w.Write([]byte(largeBody))
			require.NoError(t, err)
		})
	}

	request := func() *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		return req
	}

	t.Run("does not compress an excluded content type", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, []string{"image/png"}, upstreamWithType("image/png"))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, request())

		assert.Empty(t, rr.Header().Get("Content-Encoding"))
		assert.Equal(t, largeBody, rr.Body.String())
	})

	t.Run("still compresses other content types", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, []string{"image/png"}, upstreamWithType("text/plain"))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, request())

		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	})

	t.Run("matches by prefix", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, []string{"image/"}, upstreamWithType("image/webp"))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, request())

		assert.Empty(t, rr.Header().Get("Content-Encoding"))
	})

	t.Run("matches a content type carrying parameters", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, []string{"image/svg+xml"}, upstreamWithType("image/svg+xml; charset=utf-8"))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, request())

		assert.Empty(t, rr.Header().Get("Content-Encoding"))
	})

	t.Run("keeps the default behaviour when no exceptions are configured", func(t *testing.T) {
		handler := NewCompressionHandler(0, false, nil, upstreamWithType("image/png"))

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, request())

		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))
	})
}
