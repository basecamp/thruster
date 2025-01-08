package internal

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerGzipCompression_when_proxying(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(fixtureLength("loremipsum.txt"), 10))
		w.Write(fixtureContent("loremipsum.txt"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

	transferredSize, _ := strconv.ParseInt(w.Header().Get("Content-Length"), 10, 64)
	assert.Less(t, transferredSize, fixtureLength("loremipsum.txt"))
}

func TestHandlerGzipCompression_is_not_applied_when_not_requested(t *testing.T) {
	fixtureLength := strconv.FormatInt(fixtureLength("loremipsum.txt"), 10)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fixtureLength)
		w.Write(fixtureContent("loremipsum.txt"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Empty(t, w.Header().Get("Content-Encoding"))
	assert.Equal(t, fixtureLength, w.Header().Get("Content-Length"))
}

func TestHandlerGzipCompression_does_not_compress_images(t *testing.T) {
	fixtureLength := strconv.FormatInt(fixtureLength("image.jpg"), 10)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpg")
		w.Header().Set("Content-Length", fixtureLength)
		w.Write(fixtureContent("image.jpg"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "image/jpg")
	assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"))
	assert.Equal(t, fixtureLength, w.Header().Get("Content-Length"))
}

func TestHandlerGzipCompression_when_sendfile(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("X-Sendfile", fixturePath("loremipsum.txt"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

	transferredSize, _ := strconv.ParseInt(w.Header().Get("Content-Length"), 10, 64)
	assert.Less(t, transferredSize, fixtureLength("loremipsum.txt"))
}

func TestHandler_do_not_request_compressed_responses_from_upstream_unless_client_does(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptsGzip := r.Header.Get("Accept-Encoding") == "gzip"
		shouldAcceptGzip := r.URL.Path == "/compressed"

		assert.Equal(t, shouldAcceptGzip, acceptsGzip)
		if acceptsGzip {
			w.Header().Set("Content-Encoding", "gzip")
		}
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/plain", nil)
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"))

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/compressed", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
}

func TestHandlerMaxRequestBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()

	options := handlerOptions(upstream.URL)
	options.maxRequestBody = 10
	h := NewHandler(options)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", bytes.NewReader([]byte("Hello")))
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", bytes.NewReader([]byte("This one is too long")))
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	options.maxRequestBody = 0
	h = NewHandler(options)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/", bytes.NewReader([]byte("This one is still long")))
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlerPreserveInboundHostHeaderWhenProxying(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "example.org", r.Host)
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.org", nil)
	h.ServeHTTP(w, r)
}

func TestHandlerAppendInboundXFFHeaderWhenProxying(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "0.0.0.0, 0.0.0.1", r.Header.Get("X-Forwarded-For"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.org", nil)
	r.RemoteAddr = "0.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "0.0.0.0")
	h.ServeHTTP(w, r)
}

func TestHandlerXForwardedHeadersWhenProxying(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1.2.3.4", r.Header.Get("X-Forwarded-For"))
		assert.Equal(t, "example.org", r.Header.Get("X-Forwarded-Host"))
		assert.Equal(t, "https", r.Header.Get("X-Forwarded-Proto"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "https://example.org", nil)
	r.RemoteAddr = "1.2.3.4:1234"
	h.ServeHTTP(w, r)
}

func TestHandlerXForwardedHeadersForwardsExistingHeadersWhenForwardingEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "4.3.2.1, 1.2.3.4", r.Header.Get("X-Forwarded-For"))
		assert.Equal(t, "other.example.com", r.Header.Get("X-Forwarded-Host"))
		assert.Equal(t, "https", r.Header.Get("X-Forwarded-Proto"))
	}))
	defer upstream.Close()

	h := NewHandler(handlerOptions(upstream.URL))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.org", nil)
	r.Header.Set("X-Forwarded-For", "4.3.2.1")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "other.example.com")
	r.RemoteAddr = "1.2.3.4:1234"
	h.ServeHTTP(w, r)
}

func TestHandlerXForwardedHeadersDropsExistingHeadersWhenForwardingNotEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1.2.3.4", r.Header.Get("X-Forwarded-For"))
		assert.Equal(t, "example.org", r.Header.Get("X-Forwarded-Host"))
		assert.Equal(t, "http", r.Header.Get("X-Forwarded-Proto"))
	}))
	defer upstream.Close()

	options := handlerOptions(upstream.URL)
	options.forwardHeaders = false
	h := NewHandler(options)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.org", nil)
	r.Header.Set("X-Forwarded-For", "4.3.2.1")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "other.example.com")
	r.RemoteAddr = "1.2.3.4:1234"
	h.ServeHTTP(w, r)
}

// Helpers

func handlerOptions(targetUrl string) HandlerOptions {
	url, _ := url.Parse(targetUrl)

	return HandlerOptions{
		cache:                    NewMemoryCache(defaultCacheSize, defaultMaxCacheItemSizeBytes),
		targetUrl:                url,
		xSendfileEnabled:         true,
		maxCacheableResponseBody: 1024,
		badGatewayPage:           "",
		forwardHeaders:           true,
		logRequests:              true,
	}
}
