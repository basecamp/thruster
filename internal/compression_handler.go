package internal

import (
	"net/http"
	"strings"

	"github.com/klauspost/compress/gzhttp"
)

var compressedImageContentTypes = []string{
	"image/jpeg", "image/jpg", "image/png", "image/apng", "image/webp",
	"image/gif", "image/avif", "image/heic", "image/heif", "image/jxl",
}

func NewCompressionHandler(jitter int, disableOnAuth bool, next http.Handler) http.Handler {
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

func contentTypeFilter(contentType string) bool {
	if !gzhttp.DefaultContentTypeFilter(contentType) {
		return false
	}

	contentType = strings.ToLower(contentType)
	for _, imageType := range compressedImageContentTypes {
		if strings.HasPrefix(contentType, imageType) {
			return false
		}
	}

	return true
}
