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

func TestCacheHandler_varying(t *testing.T) {
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

// Mocks

type testCache struct {
	items map[uint64][]byte
}

func newTestCache() *testCache {
	return &testCache{items: make(map[uint64][]byte)}
}

func (t *testCache) Get(key uint64) ([]byte, bool) {
	item, found := t.items[key]
	return item, found
}

func (t *testCache) Set(key uint64, value []byte, expiresAt time.Time) {
	t.items[key] = value
}
