package internal

import (
	"log/slog"
	"net/http"
	"time"
)

const maxCacheableKeySize = 8 * KB

type RequestKey struct {
	Method string
	Host   string
	Path   string
	Query  string
	Vary   string
}

func (k RequestKey) size() int {
	return len(k.Method) + len(k.Host) + len(k.Path) + len(k.Query) + len(k.Vary)
}

func (k RequestKey) isCacheable() bool {
	return k.size() <= maxCacheableKeySize
}

type Cache interface {
	Get(key RequestKey) ([]byte, bool)
	Set(key RequestKey, value []byte, expiresAt time.Time)
}

type CacheHandler struct {
	cache       Cache
	next        http.Handler
	maxBodySize int
}

func NewCacheHandler(cache Cache, maxBodySize int, next http.Handler) *CacheHandler {
	return &CacheHandler{
		cache:       cache,
		next:        next,
		maxBodySize: maxBodySize,
	}
}

func (h *CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.shouldCacheRequest(r) {
		h.bypassCache(w, r)
		return
	}

	variant := NewVariant(r)
	response, key, found := h.fetchFromCache(r, variant)

	if found {
		variant.SetResponseHeader(response.HttpHeader)
		if !variant.Matches(response.VariantHeader) {
			response, key, found = h.fetchFromCache(r, variant)
		}
	}

	if found {
		response.WriteCachedResponse(w, r)
		return
	}

	if !key.isCacheable() {
		h.bypassCache(w, r)
		return
	}

	cr := NewCacheableResponse(w, h.maxBodySize)
	h.next.ServeHTTP(cr, r)

	cacheable, expires := cr.CacheStatus()
	if cacheable {
		variant.SetResponseHeader(cr.HttpHeader)
		cr.VariantHeader = variant.VariantHeader()

		encoded, err := cr.ToBuffer()
		if err != nil {
			slog.Error("Failed to encode response for caching", "path", r.URL.Path, "error", err)
		} else {
			h.cache.Set(key, encoded, expires)
			slog.Debug("Added response to cache", "path", r.URL.Path, "expires", expires, "size", len(encoded))
		}
	}
}

// Private

func (h *CacheHandler) bypassCache(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Bypassing cache for request", "path", r.URL.Path, "method", r.Method)
	w.Header().Set("X-Cache", "bypass")
	h.next.ServeHTTP(w, r)
}

func (h *CacheHandler) fetchFromCache(r *http.Request, variant *Variant) (CacheableResponse, RequestKey, bool) {
	key := variant.CacheKey()
	if !key.isCacheable() {
		return CacheableResponse{}, key, false
	}

	cached, found := h.cache.Get(key)

	if found {
		response, err := CacheableResponseFromBuffer(cached)
		if err != nil {
			slog.Error("Failed to decode cached response", "path", r.URL.Path, "error", err)
			return CacheableResponse{}, key, false
		}

		return response, key, true
	}

	return CacheableResponse{}, key, false
}

func (h *CacheHandler) shouldCacheRequest(r *http.Request) bool {
	allowedMethod := r.Method == http.MethodGet || r.Method == http.MethodHead
	isUpgrade := r.Header.Get("Connection") == "Upgrade" || r.Header.Get("Upgrade") == "websocket"
	isRange := r.Header.Get("Range") != ""

	return allowedMethod && !isUpgrade && !isRange
}
