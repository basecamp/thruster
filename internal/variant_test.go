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

func TestVariantCacheKey_reads_all_vary_lines(t *testing.T) {
	for name, vary := range map[string][]string{
		"accept first": {"Accept", "Cookie"},
		"cookie first": {"Cookie", "Accept"},
	} {
		t.Run(name, func(t *testing.T) {
			r1 := httptest.NewRequest("GET", "/home", nil)
			r1.Header.Set("Accept", "text/html")
			r1.Header.Set("Cookie", "session=1")

			r2 := httptest.NewRequest("GET", "/home", nil)
			r2.Header.Set("Accept", "text/html")
			r2.Header.Set("Cookie", "session=2")

			v1 := NewVariant(r1)
			v2 := NewVariant(r2)
			v1.SetResponseHeader(http.Header{"Vary": vary})
			v2.SetResponseHeader(http.Header{"Vary": vary})

			assert.Equal(t, []string{"Accept", "Cookie"}, v1.headerNames)
			assert.NotEqual(t, v1.CacheKey(), v2.CacheKey())
			assert.False(t, v1.Matches(v2.VariantHeader()))
		})
	}
}

func TestVariantCacheKey_dedupes_vary_names(t *testing.T) {
	v := NewVariant(httptest.NewRequest("GET", "/home", nil))
	v.SetResponseHeader(http.Header{"Vary": []string{"Accept, accept", "ACCEPT"}})

	assert.Equal(t, []string{"Accept"}, v.headerNames)
}

func TestVariantCacheKey_uses_all_values_of_varied_header(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/home", nil)
	r1.Header.Set("Accept", "text/html")

	r2 := httptest.NewRequest("GET", "/home", nil)
	r2.Header.Set("Accept", "text/html")
	r2.Header.Add("Accept", "application/json")

	v1 := NewVariant(r1)
	v2 := NewVariant(r2)
	v1.SetResponseHeader(http.Header{"Vary": []string{"Accept"}})
	v2.SetResponseHeader(http.Header{"Vary": []string{"Accept"}})

	assert.NotEqual(t, v1.CacheKey(), v2.CacheKey())
	assert.False(t, v1.Matches(v2.VariantHeader()))
	assert.False(t, v2.Matches(v1.VariantHeader()))
	assert.True(t, v2.Matches(v2.VariantHeader()))
}

func TestVariantCacheKey_distinguishes_absent_from_empty_header(t *testing.T) {
	r1 := httptest.NewRequest("GET", "/home", nil)

	r2 := httptest.NewRequest("GET", "/home", nil)
	r2.Header.Set("Accept", "")

	v1 := NewVariant(r1)
	v2 := NewVariant(r2)
	v1.SetResponseHeader(http.Header{"Vary": []string{"Accept"}})
	v2.SetResponseHeader(http.Header{"Vary": []string{"Accept"}})

	assert.NotEqual(t, v1.CacheKey(), v2.CacheKey())
	assert.False(t, v1.Matches(v2.VariantHeader()))
	assert.False(t, v2.Matches(v1.VariantHeader()))
	assert.Empty(t, v1.VariantHeader())
	assert.Equal(t, http.Header{"Accept": []string{""}}, v2.VariantHeader())
}
