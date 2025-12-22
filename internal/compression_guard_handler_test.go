package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/gzhttp"
	"github.com/stretchr/testify/assert"
)

func TestCompressionGuardHandler(t *testing.T) {
	tests := []struct {
		name           string
		requestHeaders map[string]string
		responseHeader map[string]string
		wantNoCompress bool
	}{
		{
			name:           "No auth headers",
			requestHeaders: map[string]string{},
			wantNoCompress: false,
		},
		{
			name:           "Cookie header present",
			requestHeaders: map[string]string{"Cookie": "session=123"},
			wantNoCompress: true,
		},
		{
			name:           "Authorization header present",
			requestHeaders: map[string]string{"Authorization": "Bearer token"},
			wantNoCompress: true,
		},
		{
			name:           "X-Csrf-Token header present",
			requestHeaders: map[string]string{"X-Csrf-Token": "token"},
			wantNoCompress: true,
		},
		{
			name:           "Set-Cookie response header",
			responseHeader: map[string]string{"Set-Cookie": "session=123"},
			wantNoCompress: true,
		},
		{
			name:           "Cache-Control private response header",
			responseHeader: map[string]string{"Cache-Control": "private, max-age=3600"},
			wantNoCompress: true,
		},
		{
			name:           "Cache-Control no-store response header",
			responseHeader: map[string]string{"Cache-Control": "no-store"},
			wantNoCompress: true,
		},
		{
			name:           "Cache-Control private directive with value",
			responseHeader: map[string]string{"Cache-Control": `public, private="Set-Cookie"`},
			wantNoCompress: true,
		},
		{
			name:           "Cache-Control token parsing avoids false positives",
			responseHeader: map[string]string{"Cache-Control": "public, my-private-setting=value"},
			wantNoCompress: false,
		},
		{
			name:           "Vary Cookie response header",
			responseHeader: map[string]string{"Vary": "Cookie"},
			wantNoCompress: true,
		},
		{
			name:           "Vary token parsing avoids false positives",
			responseHeader: map[string]string{"Vary": "Accept-Encoding, Cookie-Name"},
			wantNoCompress: false,
		},
		{
			name:           "Case-insensitive header checks (response)",
			responseHeader: map[string]string{"cache-control": "PRIVATE"},
			wantNoCompress: true,
		},
		{
			name:           "Case-insensitive header checks (request)",
			requestHeaders: map[string]string{"cookie": "session=123"},
			wantNoCompress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCompressionGuardHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.responseHeader {
					w.Header().Set(k, v)
				}
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if tt.wantNoCompress {
				assert.Equal(t, "1", rr.Header().Get(gzhttp.HeaderNoCompression))
			} else {
				assert.Empty(t, rr.Header().Get(gzhttp.HeaderNoCompression))
			}
		})
	}
}
