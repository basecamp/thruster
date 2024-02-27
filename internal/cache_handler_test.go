package internal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheHandler_caching(t *testing.T) {
	tests := map[string]struct {
		req                 *http.Request
		cacheControl        string
		expectedResponses   []string
		expectedCacheLength int
	}{
		"cacheable": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"public, max-age=60",
			[]string{"Hello 1", "Hello 1", "Hello 1"},
			1,
		},
		"cacheable with s-max-age": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"public, s-max-age=60",
			[]string{"Hello 1", "Hello 1", "Hello 1"},
			1,
		},
		"uncacheable response": {
			httptest.NewRequest("GET", "http://example.com", nil),
			"private",
			[]string{"Hello 1", "Hello 2", "Hello 3"},
			0,
		},
		"uncacheable request": {
			httptest.NewRequest("POST", "http://example.com", nil),
			"public, max-age=60",
			[]string{"Hello 1", "Hello 2", "Hello 3"},
			0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newTestCache()
			counter := 0

			handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				counter++
				w.Header().Set("Cache-Control", tc.cacheControl)
				fmt.Fprintf(w, "Hello %d", counter)
			}))

			for _, expectedResponse := range tc.expectedResponses {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, tc.req)

				assert.Equal(t, expectedResponse, w.Body.String())
			}

			assert.Equal(t, tc.expectedCacheLength, len(cache.items))
		})
	}
}

func TestCacheHandler_keying(t *testing.T) {
	tests := map[string]struct {
		paths               []string
		methods             []string
		expectedCacheLength int
	}{
		"path": {
			[]string{"http://example.com/one", "http://example.com/two", "http://example.com/three", "http://example.com/three"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet, http.MethodGet},
			3,
		},
		"query string": {
			[]string{"http://example.com?name=kevin", "http://example.com?name=kevin", "http://example.com?name=bob"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet},
			2,
		},
		"query string ordering": {
			[]string{"http://example.com?a=1&b=2", "http://example.com?a=1&b=2", "http://example.com?b=2&a=1"},
			[]string{http.MethodGet, http.MethodGet, http.MethodGet},
			1,
		},
		"method": {
			[]string{"http://example.com/one", "http://example.com/one", "http://example.com/one"},
			[]string{http.MethodGet, http.MethodHead, http.MethodPost},
			2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cache := newTestCache()

			handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "public, max-age=60")
			}))

			for i, url := range tc.paths {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(tc.methods[i], url, nil)
				handler.ServeHTTP(w, r)
			}

			assert.Equal(t, tc.expectedCacheLength, len(cache.items))
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
		w.Write([]byte(contentType))
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

func BenchmarkCacheHandler_retrieving(b *testing.B) {
	cache := NewMemoryCache(1*MB, 1*MB)

	handler := NewCacheHandler(cache, 1024, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		w.Write([]byte("Hello"))
	}))

	for i := 0; i < b.N; i++ {
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
