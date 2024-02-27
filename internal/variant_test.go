package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariantCacheKey(t *testing.T) {
	key1 := NewVariant(httptest.NewRequest("GET", "/home", nil)).CacheKey()
	key2 := NewVariant(httptest.NewRequest("GET", "/home", nil)).CacheKey()
	key3 := NewVariant(httptest.NewRequest("GET", "/home?a=b", nil)).CacheKey()
	key4 := NewVariant(httptest.NewRequest("POST", "/home?a=b", nil)).CacheKey()

	assert.Equal(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key3, key4)
}

func TestVariantCacheKey_includes_variant_header_fields(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/home", nil)
	r2 := httptest.NewRequest("GET", "/home", nil)
	r2.Header.Set("Accept-Encoding", "gzip")

	v1 := NewVariant(r1)
	v2 := NewVariant(r2)

	assert.Equal(t, v1.CacheKey(), v2.CacheKey())

	v1.SetResponseHeader(http.Header{"Vary": []string{"Accept-Encoding"}})
	v2.SetResponseHeader(http.Header{"Vary": []string{"Accept-Encoding"}})

	assert.NotEqual(t, v1.CacheKey(), v2.CacheKey())
}

func TestVariantMatches(t *testing.T) {
	r := httptest.NewRequest("GET", "/home", nil)
	r.Header.Set("Accept-Encoding", "gzip")

	v := NewVariant(r)
	v.SetResponseHeader(http.Header{"Vary": []string{"Accept-Encoding"}})

	assert.True(t, v.Matches(http.Header{"Accept-Encoding": []string{"gzip"}, "Accept": []string{"text/plain"}}))
	assert.False(t, v.Matches(http.Header{"Accept-Encoding": []string{"deflate"}}))
}

func TestVariantMatches_multiple_headers(t *testing.T) {
	r := httptest.NewRequest("GET", "/home", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	r.Header.Set("Accept", "text/plain")

	v := NewVariant(r)
	v.SetResponseHeader(http.Header{"Vary": []string{"Accept-Encoding, Accept"}})

	assert.True(t, v.Matches(http.Header{"Accept-Encoding": []string{"gzip"}, "Accept": []string{"text/plain"}}))
	assert.False(t, v.Matches(http.Header{"Accept-Encoding": []string{"gzip"}, "Accept": []string{"text/html"}}))
}

func TestVariantMatches_missing_headers(t *testing.T) {
	r := httptest.NewRequest("GET", "/home", nil)
	r.Header.Set("Accept-Encoding", "gzip")

	v := NewVariant(r)
	v.SetResponseHeader(http.Header{"Vary": []string{"Accept-Encoding, Accept"}})

	assert.True(t, v.Matches(http.Header{"Accept-Encoding": []string{"gzip"}}))
	assert.False(t, v.Matches(http.Header{"Accept-Encoding": []string{"gzip"}, "Accept": []string{"text/html"}}))
}
