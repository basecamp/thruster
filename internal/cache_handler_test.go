package internal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheHandler_caching(t *testing.T) {
	tests := map[string]struct {
		req                 *http.Request
		cacheControl        string
		expectedResponses   []string
		expectedHits        []string
		expectedCacheLength int
	}{
		"cacheable": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"public, max-age=60",
			[]string{"Hello 1", "Hello 1", "Hello 1"},
			[]string{"miss", "hit", "hit"},
			1,
		},
		"cacheable with s-max-age": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"public, s-max-age=60",
			[]string{"Hello 1", "Hello 1", "Hello 1"},
			[]string{"miss", "hit", "hit"},
			1,
		},
		"uncacheable response": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"private",
			[]string{"Hello 1", "Hello 2", "Hello 3"},
			[]string{"miss", "miss", "miss"},
			0,
		},
		"uncacheable request": {
			httptest.NewRequest("POST", "http://example.com", nil),
			"public, max-age=60",
			[]string{"Hello 1", "Hello 2", "Hello 3"},
			[]string{"bypass", "bypass", "bypass"},
			0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newTestCache()
			counter := 0
			hits := []string{}

			handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				w.Header().Set("Cache-Control", tc.cacheControl)
				fmt.Fprintf(w, "Hello %d", counter)
			}))

			for _, expectedResponse := range tc.expectedResponses {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, tc.req)
				hits = append(hits, w.Result().Header.Get("X-Cache"))

				assert.Equal(t, expectedResponse, w.Body.String())
			}

			assert.Equal(t, tc.expectedHits, hits)
			assert.Equal(t, tc.expectedCacheLength, len(cache.items))
		})
	}
}

func TestCacheHandler_keying(t *testing.T) {
	tests := map[string]struct {
		paths        []string
		methods      []string
		expectedHits []string
	}{
		"path": {
			[]string{"http://example.com/one", "http://example.com/two", "http://example.com/three", "http://example.com/three"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet, http.MethodGet},
			[]string{"miss", "miss", "miss", "hit"},
		},
		"query string": {
			[]string{"http://example.com?name=kevin", "http://example.com?name=kevin", "http://example.com?name=bob"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet},
			[]string{"miss", "hit", "miss"},
		},
		"query string ordering": {
			[]string{"http://example.com?a=1&b=2", "http://example.com?a=1&b=2", "http://example.com?b=2&a=1"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet},
			[]string{"miss", "hit", "hit"},
		},
		"method": {
			[]string{"http://example.com/one", "http://example.com/one", "http://example.com/one"},
			[]string{http.MethodGet, http.MethodHead, http.MethodPost},
			[]string{"miss", "miss", "bypass"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newTestCache()
			handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "public, max-age=60")
				_, _ = w.Write([]byte("Hello"))
			}))

			hits := []string{}

			for i, url := range tc.paths {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(tc.methods[i], url, nil)
				handler.ServeHTTP(w, r)

				hits = append(hits, w.Result().Header.Get("X-Cache"))
			}

			assert.Equal(t, tc.expectedHits, hits)
		})
	}
}

func TestCacheHandler_vary_header(t *testing.T) {
	cache := newTestCache()
	handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Accept")
		w.Header().Set("Vary", "Accept")
		w.Header().Set("Cache-Control", "public, max-age=600")
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(contentType))
	}))

	doReq := func(accept string, other string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://example.com", nil)
		r.Header.Set("Accept", accept)
		r.Header.Set("Other", other)
		handler.ServeHTTP(w, r)
		return w
	}

	resp := doReq("application/json", "a")
	assert.Equal(t, "application/json", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("application/json", "b")
	assert.Equal(t, "application/json", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))

	resp = doReq("text/plain", "a")
	assert.Equal(t, "text/plain", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("text/plain", "a")
	assert.Equal(t, "text/plain", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))

	resp = doReq("application/json", "b")
	assert.Equal(t, "application/json", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))
}

func TestCacheHandler_different_hosts(t *testing.T) {
	cache := newTestCache()
	handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Header.Get("Host")
		w.Header().Set("Cache-Control", "public, max-age=600")
		_, _ = w.Write([]byte(host))
	}))

	doReq := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", url, nil)
		host := strings.Split(url, "://")[1]
		r.Header.Set("Host", host)
		handler.ServeHTTP(w, r)
		return w
	}

	resp := doReq("https://example.com")
	assert.Equal(t, "example.com", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("https://example.com")
	assert.Equal(t, "example.com", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))

	resp = doReq("https://another.com")
	assert.Equal(t, "another.com", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("https://another.com")
	assert.Equal(t, "another.com", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))

	resp = doReq("https://example.com/test")
	assert.Equal(t, "example.com/test", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("https://another.com/test")
	assert.Equal(t, "another.com/test", resp.Body.String())
	assert.Equal(t, "miss", resp.Header().Get("X-Cache"))

	resp = doReq("https://another.com/test")
	assert.Equal(t, "another.com/test", resp.Body.String())
	assert.Equal(t, "hit", resp.Header().Get("X-Cache"))
}

func TestCacheHandler_range_requests_are_not_cached(t *testing.T) {
	cache := newTestCache()

	handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		http.ServeFile(w, r, fixturePath("image.jpg"))
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Range", "bytes=0-1")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "2", w.Header().Get("Content-Length"))
	assert.Equal(t, fixtureContent("image.jpg")[:2], w.Body.Bytes())
	assert.Equal(t, "bypass", w.Header().Get("X-Cache"))

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Range", "bytes=2-5")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "4", w.Header().Get("Content-Length"))
	assert.Equal(t, fixtureContent("image.jpg")[2:6], w.Body.Bytes())
	assert.Equal(t, "bypass", w.Header().Get("X-Cache"))
}

func BenchmarkCacheHandler_retrieving(b *testing.B) {
	cache := NewMemoryCache(1*MB, 1*MB)

	handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		_, _ = w.Write([]byte("Hello"))
	}))

	for b.Loop() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		handler.ServeHTTP(w, r)
	}
}

// Mocks

type testCache struct {
	items map[CacheKey][]byte
}

func newTestCache() *testCache {
	return &testCache{items: make(map[CacheKey][]byte)}
}

func (t *testCache) Get(key CacheKey) ([]byte, bool) {
	item, found := t.items[key]
	return item, found
}

func (t *testCache) Set(key CacheKey, value []byte, expiresAt time.Time) {
	t.items[key] = value
}
