package internal

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendfileHandler(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		w.Write([]byte("This body should not be seen"))
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, strconv.FormatInt(fixtureLength("image.jpg"), 10), w.Header().Get("Content-Length"))
	assert.Equal(t, fixtureContent("image.jpg"), w.Body.Bytes())
}

func TestSendFileHandler_when_no_x_sendfile_present(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("Content-Type", "application/custom")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("This body should be seen"))
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "application/custom", w.Header().Get("Content-Type"))
	assert.Equal(t, "This body should be seen", w.Body.String())
}

func TestSendFileHandler_when_not_enabled(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("Content-Type", "application/custom")
		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("This body should be seen"))
	}

	h := NewSendfileHandler(false, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "application/custom", w.Header().Get("Content-Type"))
	assert.Equal(t, "This body should be seen", w.Body.String())
}
