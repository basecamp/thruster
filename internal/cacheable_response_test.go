package internal

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheableResponse_cache_headers(t *testing.T) {
	tests := map[string]struct {
		cacheControl string
		cacheable    bool
	}{
		"public, with max-age": {
			cacheControl: "public, max-age=60",
			cacheable:    true,
		},

		"public, with s-max-age": {
			cacheControl: "public, s-max-age=60",
			cacheable:    true,
		},

		"public, with max-age of zero": {
			cacheControl: "public, max-age=0",
			cacheable:    false,
		},

		"public, with no max-age": {
			cacheControl: "public",
			cacheable:    false,
		},

		"private, with max-age": {
			cacheControl: "private, max-age=60",
			cacheable:    false,
		},

		"max-age, but no public specified": {
			cacheControl: "max-age=60",
			cacheable:    false,
		},

		"public, with max-age, but also no-cache": {
			cacheControl: "public, max-age=60, no-cache",
			cacheable:    false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			cr := NewCacheableResponse(rec, 1024)
			cr.Header().Set("Cache-Control", test.cacheControl)

			cacheable, _ := cr.CacheStatus()
			assert.Equal(t, test.cacheable, cacheable)
		})
	}
}

func TestCacheableResponse_does_not_cache_items_with_vary_header(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Set("Vary", "Accept-Encoding")

	cacheable, _ := cr.CacheStatus()
	assert.False(t, cacheable)
}

func TestCacheableResponse_does_not_cache_items_where_body_too_large(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 10)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Write([]byte("12345678901234567890"))

	cacheable, _ := cr.CacheStatus()
	assert.False(t, cacheable)
}

func TestCacheableResponse_does_not_cache_304_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusNotModified)

	cacheable, _ := cr.CacheStatus()
	assert.False(t, cacheable)
}

func TestCacheableResponse_writes_response_to_writer(t *testing.T) {
	w := httptest.NewRecorder()
	cr := NewCacheableResponse(w, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusCreated)
	cr.Write([]byte("Hello World"))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
	assert.Equal(t, "miss", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_writes_response_to_writer_even_when_too_large_to_cache(t *testing.T) {
	w := httptest.NewRecorder()
	cr := NewCacheableResponse(w, 10)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusCreated)
	cr.Write([]byte("12345678901234567890"))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "12345678901234567890", w.Body.String())
	assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
	assert.Equal(t, "miss", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_write_cached_response(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusCreated)
	cr.Write([]byte("Hello World"))

	cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	cr.WriteCachedResponse(w)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_scrubs_cookies_from_cacheable_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Set("Set-Cookie", "user=1234; Path=/; HttpOnly")
	cr.WriteHeader(http.StatusOK)

	w := httptest.NewRecorder()

	cr.WriteCachedResponse(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Set-Cookie"))
}

func TestCacheableResponse_does_not_scrub_cookies_from_non_cacheable_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Set-Cookie", "user=1234; Path=/; HttpOnly")
	cr.WriteHeader(http.StatusOK)

	w := httptest.NewRecorder()

	cr.WriteCachedResponse(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user=1234; Path=/; HttpOnly", w.Header().Get("Set-Cookie"))
}

func TestCacheableResponse_serialization(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusCreated)
	cr.Write([]byte("Hello World"))

	saved, err := cr.ToBuffer()
	assert.NoError(t, err)

	restored, err := CacheableResponseFromBuffer(saved)
	assert.NoError(t, err)

	assert.Equal(t, cr.StatusCode, restored.StatusCode)
	assert.Equal(t, cr.Header(), restored.Header())
	assert.Equal(t, cr.Body, restored.Body)
}

func TestStashingWriter_writing_within_limit(t *testing.T) {
	writer := &bytes.Buffer{}
	sw := NewStashingWriter(10, writer)

	written, err := sw.Write([]byte("12345"))
	require.NoError(t, err)
	assert.Equal(t, 5, written)

	assert.Equal(t, "12345", writer.String())
	assert.Equal(t, []byte("12345"), sw.Body())
	assert.False(t, sw.Overflowed())
}

func TestStashingWriter_writing_over_limit(t *testing.T) {
	writer := &bytes.Buffer{}
	sw := NewStashingWriter(10, writer)

	written, err := sw.Write([]byte("12345678901234567890"))
	require.NoError(t, err)
	assert.Equal(t, 20, written)

	assert.Equal(t, "12345678901234567890", writer.String())
	assert.Nil(t, sw.Body())
	assert.True(t, sw.Overflowed())
}

func TestStashingWriter_writing_over_limit_in_small_pieces(t *testing.T) {
	writer := &bytes.Buffer{}
	sw := NewStashingWriter(10, writer)

	for i := 0; i < 10; i++ {
		written, err := sw.Write([]byte("12"))
		require.NoError(t, err)
		assert.Equal(t, 2, written)
	}

	assert.Equal(t, "12121212121212121212", writer.String())
	assert.Nil(t, sw.Body())
	assert.True(t, sw.Overflowed())
}
