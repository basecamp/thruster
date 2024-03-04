package internal

import (
	"bytes"
	"image"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

var allowedFormats = []string{"gif", "jpeg", "png", "webp"}

const (
	imageProxyHandlerPath  = "/_t/image"
	imageProxyMaxDimension = 5000
)

type ImageProxyHandler struct {
	httpClient *http.Client
}

func RegisterNewImageProxyHandler(mux *http.ServeMux) {
	handler := &ImageProxyHandler{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	mux.Handle("GET "+imageProxyHandlerPath, handler)
}

func (h *ImageProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	remoteURL := h.extractRemoteURL(r)
	if remoteURL == nil {
		http.Error(w, "invalid url", http.StatusNotFound)
		return
	}

	resp, err := h.httpClient.Get(remoteURL.String())
	if err != nil {
		http.Error(w, "error fetching remote image", http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		h.copyHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)
		return
	}

	imageReader := h.sanitizeImage(resp.Body)
	if imageReader == nil {
		http.Error(w, "invalid image", http.StatusForbidden)
		return
	}

	slog.Info("Proxying remote image", "url", remoteURL)

	h.copyHeaders(w, resp)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, imageReader)
}

// Private

func (h *ImageProxyHandler) extractRemoteURL(r *http.Request) *url.URL {
	urlString := r.URL.Query().Get("src")
	if urlString == "" {
		return nil
	}

	remoteURL, err := url.Parse(urlString)
	if err != nil || (remoteURL.Scheme != "http" && remoteURL.Scheme != "https") {
		return nil
	}

	return remoteURL
}

func (h *ImageProxyHandler) copyHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
}

func (h *ImageProxyHandler) sanitizeImage(f io.Reader) io.Reader {
	var buf bytes.Buffer
	reader := io.TeeReader(f, &buf)

	cfg, format, err := image.DecodeConfig(reader)
	if err != nil {
		slog.Debug("ImageProxy: image format not valid", "err", err)
		return nil
	}

	if !slices.Contains(allowedFormats, format) {
		slog.Debug("ImageProxy: image format not allowed", "format", format)
		return nil
	}

	if cfg.Width > imageProxyMaxDimension || cfg.Height > imageProxyMaxDimension {
		slog.Debug("ImageProxy: image too large", "width", cfg.Width, "height", cfg.Height)
		return nil
	}

	slog.Debug("ImageProxy: image acceptable", "format", format, "width", cfg.Width, "height", cfg.Height)
	return io.MultiReader(&buf, f)
}
