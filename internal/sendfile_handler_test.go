package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendfileHandler(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		_, _ = w.Write([]byte("This body should not be seen"))
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

func TestSendfileHandler_sends_correct_content_length_when_content_encoding_present(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", "0")
		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, fixtureContent("image.jpg"), w.Body.Bytes())
	assert.Equal(t, strconv.FormatInt(fixtureLength("image.jpg"), 10), w.Header().Get("Content-Length"))
}

func TestSendfileHandler_range_request_when_content_encoding_present(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", "0")
		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Range", "bytes=0-9")
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Equal(t, "10", w.Header().Get("Content-Length"))
	assert.Equal(t, fixtureContent("image.jpg")[:10], w.Body.Bytes())
}

func TestSendfileHandler_precondition_failed_has_no_content_length(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sendfile", fixturePath("image.jpg"))
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("If-Match", `"nope"`)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	assert.Empty(t, w.Header().Get("Content-Length"))
	assert.Empty(t, w.Body.Bytes())
}

func TestSendFileHandler_when_no_x_sendfile_present(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "X-Sendfile", r.Header.Get("X-Sendfile-Type"))

		w.Header().Set("Content-Type", "application/custom")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("This body should be seen"))
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
		_, _ = w.Write([]byte("This body should be seen"))
	}

	h := NewSendfileHandler(false, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "application/custom", w.Header().Get("Content-Type"))
	assert.Equal(t, "This body should be seen", w.Body.String())
}

func TestSendfileHandler_serves_files_named_index_html(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index.html")
	require.NoError(t, os.WriteFile(filename, []byte("<html>index</html>"), 0644))

	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sendfile", filename)
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/some/dir/index.html", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "18", w.Header().Get("Content-Length"))
	assert.Equal(t, "<html>index</html>", w.Body.String())
}

func TestSendfileHandler_when_file_is_missing(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("X-Sendfile", filepath.Join(t.TempDir(), "missing.txt"))
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, w.Header().Get("Content-Encoding"))
}

func TestSendfileHandler_when_file_is_a_directory(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sendfile", t.TempDir())
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSendfileHandler_when_path_component_is_a_file(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sendfile", filepath.Join(fixturePath("image.jpg"), "extra"))
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSendfileHandler_when_file_is_not_readable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}

	filename := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(filename, []byte("secret"), 0000))

	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sendfile", filename)
		w.WriteHeader(http.StatusOK)
	}

	h := NewSendfileHandler(true, http.HandlerFunc(upstream))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
