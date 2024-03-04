package internal

import (
	"image"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageProxy_serving_valid_images(t *testing.T) {
	tests := map[string]struct {
		filename   string
		statusCode int
	}{
		"valid gif":    {"image.gif", http.StatusOK},
		"valid jpg":    {"image.jpg", http.StatusOK},
		"valid png":    {"image.png", http.StatusOK},
		"valid webp":   {"image.webp", http.StatusOK},
		"valid svg":    {"image.svg", http.StatusForbidden},
		"not an image": {"loremipsum.txt", http.StatusForbidden},
		"missing file": {"doesnotexist.txt", http.StatusNotFound},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !fixtureExists(tc.filename) {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.Write(fixtureContent(tc.filename))
			}))
			defer remoteServer.Close()

			mux := http.NewServeMux()
			RegisterNewImageProxyHandler(mux)
			localServer := httptest.NewServer(mux)
			defer localServer.Close()

			imageURL, _ := url.Parse(localServer.URL + imageProxyHandlerPath)
			params := url.Values{}
			params.Add("src", remoteServer.URL)
			imageURL.RawQuery = params.Encode()

			resp, err := http.Get(imageURL.String())

			require.NoError(t, err)
			assert.Equal(t, tc.statusCode, resp.StatusCode)

			if tc.statusCode == http.StatusOK {
				_, _, err = image.Decode(resp.Body)
				require.NoError(t, err)
			}
		})
	}
}
