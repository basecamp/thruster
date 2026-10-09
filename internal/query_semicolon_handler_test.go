package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuerySemicolonHandler_encodes_semicolons_when_enabled(t *testing.T) {
	tests := map[string]string{
		"/?a=1;2&b=3":   "a=1%3B2&b=3",
		"/?a=1%3B2&b=3": "a=1%3B2&b=3",
		"/?a=1%3b2&b=3": "a=1%3b2&b=3",
		"/?a=1;;2":      "a=1%3B%3B2",
	}

	for target, expected := range tests {
		t.Run(target, func(t *testing.T) {
			var rawQuery, value string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rawQuery = r.URL.RawQuery
				value = r.URL.Query().Get("a")
			}))
			defer upstream.Close()

			options := handlerOptions(upstream.URL)
			options.encodeQuerySemicolons = true
			h := NewHandler(options)

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", target, nil))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, expected, rawQuery)
			assert.Contains(t, value, ";")
		})
	}
}

func TestQuerySemicolonHandler_drops_semicolon_pairs_when_disabled(t *testing.T) {
	var rawQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/?a=1;2&b=3", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "b=3", rawQuery)
}

func TestQuerySemicolonHandler_keeps_cache_keys_distinct(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte(r.URL.Query().Get("a")))
	}))
	defer upstream.Close()

	options := handlerOptions(upstream.URL)
	options.encodeQuerySemicolons = true
	h := NewHandler(options)

	serveRequest := func(target string) (string, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		return w.Header().Get("X-Cache"), w.Body.String()
	}

	cache, body := serveRequest("/?a=1;2")
	assert.Equal(t, "miss", cache)
	assert.Equal(t, "1;2", body)

	cache, body = serveRequest("/")
	assert.Equal(t, "miss", cache)
	assert.Equal(t, "", body)

	cache, body = serveRequest("/?a=1%3B2")
	assert.Equal(t, "hit", cache)
	assert.Equal(t, "1;2", body)
}
