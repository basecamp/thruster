package internal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaxRequestBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	limitedServer := httptest.NewServer(NewMaxRequestBodyHandler(10, handler))
	defer limitedServer.Close()

	unlimitedServer := httptest.NewServer(NewMaxRequestBodyHandler(0, handler))
	defer unlimitedServer.Close()

	resp, err := http.Post(limitedServer.URL, "text/plain", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post(limitedServer.URL, "text/plain", strings.NewReader("1234567890"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = http.Post(limitedServer.URL, "text/plain", strings.NewReader("this is too long"))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, err = http.Post(unlimitedServer.URL, "text/plain", strings.NewReader("this is long but the limit is not enforced"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
