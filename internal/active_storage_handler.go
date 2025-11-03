package internal

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/basecamp/thruster/internal/active_storage"
)

type ActiveStorageHandler struct {
	secret string
	next   http.Handler
}

func NewActiveStorageHandler(secret string, next http.Handler) *ActiveStorageHandler {
	return &ActiveStorageHandler{
		secret: secret,
		next:   next,
	}
}

func (h *ActiveStorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if active_storage.IsRepresentationPath(r.URL.Path) {
		slog.Debug("Intercepting Active Storage representation request", "path", r.URL.Path)

		representation, err := active_storage.ParseRepresentationFromPath(r.URL.Path)
		if err != nil {
			slog.Warn("Failed to parse Active Storage path", "path", r.URL.Path, "error", err)
		} else {
			slog.Debug("Parsed Active Storage request", "blobKey", representation.BlobKey, "variation", representation.Variation, "filename", representation.Filename)
			h.handleActiveStorageRepresentation(w, r, representation)
			return
		}
	}

	h.next.ServeHTTP(w, r)
}

func (h *ActiveStorageHandler) handleActiveStorageRepresentation(w http.ResponseWriter, r *http.Request, representation *active_storage.Representation) {
	// TODO: Synchronize multiple concurrent requests such that
	// only the first one processes the image, and all other wait
	// for it to be cached
	
	// TODO: Check if representation exists in cache,
	// if it does serve it from ther
	
	processed, err := representation.Process(h.secret)
	if err != nil {
		slog.Error("Failed to generate representation", "blobKey", representation.BlobKey, "error", err)
		http.Error(w, "Failed to process file", http.StatusInternalServerError)
		return
	}
	defer processed.Close()

	// TODO: Store the result in cache

	w.Header().Set("Content-Type", processed.ContentType)
	w.Header().Set("X-Handled-By", "Thruster-ActiveStorage")
	w.WriteHeader(http.StatusOK)

	bytesWritten, err := io.Copy(w, processed.Reader)
	if err != nil {
		slog.Error("Failed to write response", "error", err)
		return
	}

	slog.Info("Served Active Storage representation",
		"blobKey", representation.BlobKey,
		"variation", representation.Variation,
		"filename", representation.Filename,
		"bytesWritten", bytesWritten)
}
