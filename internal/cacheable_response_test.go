package internal

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

		"public, with s-maxage": {
			cacheControl: "public, s-maxage=60",
			cacheable:    true,
		},

		"public, with misspelled s-max-age": {
			cacheControl: "public, s-max-age=60",
			cacheable:    false,
		},

		"public, with s-maxage of zero and a max-age": {
			cacheControl: "public, s-maxage=0, max-age=60",
			cacheable:    false,
		},

		"public, with quoted max-age": {
			cacheControl: `public, max-age="60"`,
			cacheable:    true,
		},

		"public, with doubly quoted max-age": {
			cacheControl: `public, max-age=""60""`,
			cacheable:    false,
		},

		"public, with half quoted max-age": {
			cacheControl: `public, max-age="60`,
			cacheable:    false,
		},

		"public, with max-age, but also no-store": {
			cacheControl: "public, max-age=60, no-store",
			cacheable:    false,
		},

		"public, with max-age, but also private": {
			cacheControl: "public, private, max-age=60",
			cacheable:    false,
		},

		"public, with max-age, but also private with a field name": {
			cacheControl: `public, private="Set-Cookie", max-age=60`,
			cacheable:    false,
		},

		"public, with max-age, in mixed case": {
			cacheControl: "Public, Max-Age=60",
			cacheable:    true,
		},

		"public inside a quoted extension value": {
			cacheControl: `ext="a, public, b", max-age=60`,
			cacheable:    false,
		},

		"max-age inside a quoted extension value": {
			cacheControl: `public, ext="a, max-age=60"`,
			cacheable:    false,
		},

		"public, with duplicate max-age": {
			cacheControl: "public, max-age=0, max-age=600",
			cacheable:    false,
		},

		"public, with negative max-age that overflows a duration": {
			cacheControl: "public, max-age=-10000000000",
			cacheable:    false,
		},

		"public, with max-age too large for a duration": {
			cacheControl: "public, max-age=19000000000",
			cacheable:    true,
		},

		"public, with non-numeric max-age": {
			cacheControl: "public, max-age=soon",
			cacheable:    false,
		},

		"public, with signed max-age": {
			cacheControl: "public, max-age=+60",
			cacheable:    false,
		},

		"public, with an unterminated quoted value hiding no-store": {
			cacheControl: `public, max-age=60, ext="unterminated, no-store`,
			cacheable:    false,
		},

		"public, with an escaped quote inside a quoted value": {
			cacheControl: `public, ext="a \" b", max-age=60`,
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

func TestCacheableResponse_cache_headers_across_lines(t *testing.T) {
	tests := map[string]struct {
		cacheControl []string
		cacheable    bool
	}{
		"public and max-age on separate lines": {
			cacheControl: []string{"public", "max-age=60"},
			cacheable:    true,
		},

		"no-store on a later line": {
			cacheControl: []string{"public, max-age=60", "no-store"},
			cacheable:    false,
		},

		"private on a later line": {
			cacheControl: []string{"public, max-age=60", "private"},
			cacheable:    false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			cr := NewCacheableResponse(rec, 1024)
			cr.Header()["Cache-Control"] = test.cacheControl

			cacheable, _ := cr.CacheStatus()
			assert.Equal(t, test.cacheable, cacheable)
		})
	}
}

func TestCacheableResponse_caps_very_long_lifetimes(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=19000000000")

	cacheable, expires := cr.CacheStatus()
	assert.True(t, cacheable)
	assert.WithinDuration(t, time.Now().Add(maxCacheLifetime), expires, time.Second)
}

func TestCacheableResponse_s_maxage_takes_precedence_over_max_age(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, s-maxage=10, max-age=3600")

	cacheable, expires := cr.CacheStatus()
	assert.True(t, cacheable)
	assert.WithinDuration(t, time.Now().Add(10*time.Second), expires, time.Second)
}

func TestCacheableResponse_does_not_cache_items_with_wildcard_vary_header(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Set("Vary", "*")

	cacheable, _ := cr.CacheStatus()
	assert.False(t, cacheable)
}

func TestCacheableResponse_does_not_cache_items_with_wildcard_vary_on_a_later_line(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Add("Vary", "Accept")
	cr.Header().Add("Vary", "*")

	cacheable, _ := cr.CacheStatus()
	assert.False(t, cacheable)
}

func TestCacheableResponse_does_not_cache_items_that_vary_on_proxy_set_headers(t *testing.T) {
	for _, name := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "x-forwarded-for"} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			cr := NewCacheableResponse(rec, 1024)
			cr.Header().Set("Cache-Control", "public, max-age=60")
			cr.Header().Set("Vary", "Accept, "+name)

			cacheable, _ := cr.CacheStatus()
			assert.False(t, cacheable)
		})
	}
}

func TestCacheableResponse_does_not_cache_items_where_body_too_large(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 10)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = cr.Write([]byte("12345678901234567890"))

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
	_, _ = cr.Write([]byte("Hello World"))

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
	_, _ = cr.Write([]byte("12345678901234567890"))

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
	_, _ = cr.Write([]byte("Hello World"))

	_, _ = cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Equal(t, "public, max-age=60", w.Header().Get("Cache-Control"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_conditional_response(t *testing.T) {
	etag := `"deadbeef"`

	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Etag", etag)
	cr.WriteHeader(http.StatusOK)
	_, _ = cr.Write([]byte("Hello World"))

	_, _ = cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", etag)
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusNotModified, w.Code)
	assert.Equal(t, "", w.Body.String())
	assert.Equal(t, etag, w.Header().Get("Etag"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", "\"another\", \"deadbeef\"")
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusNotModified, w.Code)
	assert.Equal(t, "", w.Body.String())
	assert.Equal(t, etag, w.Header().Get("Etag"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_conditional_response_none_match(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Etag", "ffffffff")
	cr.WriteHeader(http.StatusOK)
	_, _ = cr.Write([]byte("Hello World"))

	_, _ = cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", "deadbeef")
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Equal(t, "ffffffff", w.Header().Get("Etag"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_conditional_response_no_etag_in_request(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Etag", "ffffffff")
	cr.WriteHeader(http.StatusOK)
	_, _ = cr.Write([]byte("Hello World"))

	_, _ = cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Equal(t, "ffffffff", w.Header().Get("Etag"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_conditional_response_no_etag_in_response(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.WriteHeader(http.StatusOK)
	_, _ = cr.Write([]byte("Hello World"))

	_, _ = cr.ToBuffer() // Ensure the body is saved

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("If-None-Match", "deadbeef")
	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello World", w.Body.String())
	assert.Empty(t, w.Header().Get("Etag"))
	assert.Equal(t, "hit", w.Header().Get("X-Cache"))
}

func TestCacheableResponse_scrubs_cookies_from_cacheable_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Set("Set-Cookie", "user=1234; Path=/; HttpOnly")
	cr.WriteHeader(http.StatusOK)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Set-Cookie"))
}

func TestCacheableResponse_does_not_scrub_cookies_from_non_cacheable_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Set-Cookie", "user=1234; Path=/; HttpOnly")
	cr.WriteHeader(http.StatusOK)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	cr.WriteCachedResponse(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user=1234; Path=/; HttpOnly", w.Header().Get("Set-Cookie"))
}

func TestCacheableResponse_does_not_scrub_cookies_from_no_store_responses(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, no-store, max-age=60")
	cr.Header().Set("Set-Cookie", "user=1234; Path=/; HttpOnly")
	cr.WriteHeader(http.StatusOK)

	assert.Equal(t, "user=1234; Path=/; HttpOnly", rec.Header().Get("Set-Cookie"))
}

func TestCacheableResponse_serialization(t *testing.T) {
	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.WriteHeader(http.StatusCreated)
	_, _ = cr.Write([]byte("Hello World"))

	saved, err := cr.ToBuffer()
	assert.NoError(t, err)

	restored, err := CacheableResponseFromBuffer(saved)
	assert.NoError(t, err)

	assert.Equal(t, cr.StatusCode, restored.StatusCode)
	assert.Equal(t, cr.Header(), restored.Header())
	assert.Equal(t, cr.Body, restored.Body)
}

func TestCacheableResponse_serialization_of_variant_header(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header["Accept"] = []string{"text/html", ""}

	rec := httptest.NewRecorder()
	cr := NewCacheableResponse(rec, 1024)
	cr.Header().Set("Cache-Control", "public, max-age=60")
	cr.Header().Set("Vary", "Accept, Cookie")
	cr.WriteHeader(http.StatusOK)

	variant := NewVariant(r)
	variant.SetResponseHeader(cr.Header())
	cr.VariantHeader = variant.VariantHeader()

	saved, err := cr.ToBuffer()
	assert.NoError(t, err)

	restored, err := CacheableResponseFromBuffer(saved)
	assert.NoError(t, err)

	assert.Equal(t, http.Header{"Accept": []string{"text/html", ""}}, restored.VariantHeader)
	assert.True(t, variant.Matches(restored.VariantHeader))

	other := httptest.NewRequest(http.MethodGet, "/", nil)
	other.Header["Accept"] = []string{"text/html", ""}
	other.Header.Set("Cookie", "session=1")
	otherVariant := NewVariant(other)
	otherVariant.SetResponseHeader(cr.Header())
	assert.False(t, otherVariant.Matches(restored.VariantHeader))
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

	for range 10 {
		written, err := sw.Write([]byte("12"))
		require.NoError(t, err)
		assert.Equal(t, 2, written)
	}

	assert.Equal(t, "12121212121212121212", writer.String())
	assert.Nil(t, sw.Body())
	assert.True(t, sw.Overflowed())
}
