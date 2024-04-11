package internal

import (
	"log/slog"
	"net/http"
	"time"
)

type CacheKey uint64

type Cache interface {
	Get(key CacheKey) ([]byte, bool)
	Set(key CacheKey, value []byte, expiresAt time.Time)
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

	if !h.shouldCacheRequest(r) {
		slog.Debug("Bypassing cache for request", "path", r.URL.Path, "method", r.Method)
		w.Header().Set("X-Cache", "bypass")
		h.next.ServeHTTP(w, r)
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
			slog.Debug("Added response to cache", "path", r.URL.Path, "key", key, "expires", expires, "size", len(encoded))
		}
	}
}

// Private

func (h *CacheHandler) fetchFromCache(r *http.Request, variant *Variant) (CacheableResponse, CacheKey, bool) {
	key := variant.CacheKey()
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
