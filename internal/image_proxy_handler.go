package internal

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/basecamp/thruster/internal/image_proxy"
)

const (
	imageProxyPathPrefix = "/thruster/image_proxy/"
)

type ImageProxyHandler struct {
	secret string
	next   http.Handler
}

func NewImageProxyHandler(secret string, next http.Handler) *ImageProxyHandler {
	return &ImageProxyHandler{
		secret: secret,
		next:   next,
	}
}

func (h *ImageProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, imageProxyPathPrefix) {
		data := strings.TrimPrefix(r.URL.Path, imageProxyPathPrefix)

		slog.Info("Processing image proxy request", "representation", data)

		representation, err := image_proxy.ParseRepresentation(h.secret, data)
		if err != nil {
			slog.Error("Failed to parse file representation", "path", r.URL.Path, "error", err)
			http.Error(w, "Invalid representation", http.StatusBadRequest)
			return
		}

		slog.Info("Parsed representation", "representation", representation)
		h.handleRepresentation(w, r, representation)
		return
	}

	h.next.ServeHTTP(w, r)
}

func (h *ImageProxyHandler) handleRepresentation(w http.ResponseWriter, r *http.Request, representation *image_proxy.Representation) {
	// TODO: Synchronize multiple concurrent requests such that
	// only the first one processes the image, and all other wait
	// for it to be cached

	// TODO: Check if representation exists in cache,
	// if it does serve it from there

	processed, err := representation.Process()
	if err != nil {
		slog.Error("Failed to generate representation", "filename", representation.Filename, "error", err)
		http.Error(w, "Failed to process file", http.StatusInternalServerError)
		return
	}
	defer processed.Close()

	// TODO: Store the result in cache

	w.Header().Set("Content-Type", processed.ContentType)
	w.Header().Set("X-Handled-By", "Thruster-ImageProxy")
	w.WriteHeader(http.StatusOK)

	bytesWritten, err := io.Copy(w, processed.Reader)
	if err != nil {
		slog.Error("Failed to write response", "error", err)
		return
	}

	slog.Info("Served image proxy representation",
		"filename", representation.Filename,
		"contentType", representation.ContentType,
		"bytesWritten", bytesWritten)
}
