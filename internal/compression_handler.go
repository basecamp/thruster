package internal

import (
	"net/http"
	"strings"

	"github.com/klauspost/compress/gzhttp"
)

func NewCompressionHandler(jitter int, disableOnAuth bool, exceptContentTypes []string, next http.Handler) http.Handler {
	contentTypeFilter := gzhttp.DefaultContentTypeFilter
	if len(exceptContentTypes) > 0 {
		contentTypeFilter = newExceptContentTypeFilter(exceptContentTypes)
	}

	var wrapper func(http.Handler) http.HandlerFunc
	var err error

	if jitter > 0 {
		wrapper, err = gzhttp.NewWrapper(
			gzhttp.MinSize(1024),
			gzhttp.CompressionLevel(6),
			gzhttp.ContentTypeFilter(contentTypeFilter),
			gzhttp.RandomJitter(jitter, 0, false),
		)
	} else {
		wrapper, err = gzhttp.NewWrapper(
			gzhttp.MinSize(1024),
			gzhttp.CompressionLevel(6),
			gzhttp.ContentTypeFilter(contentTypeFilter),
		)
	}

	if err != nil {
		panic("failed to create gzip wrapper: " + err.Error())
	}

	handler := wrapper(next)

	if disableOnAuth {
		return NewCompressionGuardHandler(handler)
	}

	return handler
}

// newExceptContentTypeFilter extends gzhttp's default filter so that any
// content type matching one of the configured prefixes is left uncompressed.
// Everything else keeps the default behaviour, so an empty list is a no-op.
func newExceptContentTypeFilter(exceptContentTypes []string) func(string) bool {
	prefixes := make([]string, 0, len(exceptContentTypes))
	for _, contentType := range exceptContentTypes {
		contentType = strings.TrimSpace(strings.ToLower(contentType))
		if contentType != "" {
			prefixes = append(prefixes, contentType)
		}
	}

	return func(contentType string) bool {
		if !gzhttp.DefaultContentTypeFilter(contentType) {
			return false
		}

		normalized := strings.TrimSpace(strings.ToLower(contentType))
		for _, prefix := range prefixes {
			if strings.HasPrefix(normalized, prefix) {
				return false
			}
		}

		return true
	}
}
