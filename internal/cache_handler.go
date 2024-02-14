package internal

import (
	"hash/fnv"
	"log/slog"
	"net/http"
	"time"
)

type Cache interface {
	Get(key uint64) ([]byte, bool)
	Set(key uint64, value []byte, expiresAt time.Time)
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
	key := h.deriveCacheKey(r)
	cached, found := h.cache.Get(key)

	if found {
		response, err := CacheableResponseFromBuffer(cached)
		if err != nil {
			slog.Error("Failed to decode cached response", "path", r.URL.Path, "error", err)
		} else {
			response.WriteCachedResponse(w)
			return
		}
	}

	if !h.shouldCacheRequest(r) {
		h.next.ServeHTTP(w, r)
		return
	}

	cr := NewCacheableResponse(w, h.maxBodySize)
	h.next.ServeHTTP(cr, r)

	cacheable, expires := cr.CacheStatus()
	if cacheable {
		encoded, err := cr.ToBuffer()
		if err != nil {
			slog.Error("Failed to encode response for caching", "path", r.URL.Path, "error", err)
		} else {
			h.cache.Set(key, encoded, expires)
			slog.Info("Cached response", "path", r.URL.Path, "expires", expires)
		}
	}
}

func (h *CacheHandler) deriveCacheKey(r *http.Request) uint64 {
	hash := fnv.New64()
	hash.Write([]byte(r.Method))
	hash.Write([]byte(r.URL.Path))
	hash.Write([]byte(r.URL.Query().Encode()))
	return hash.Sum64()
}

func (h *CacheHandler) shouldCacheRequest(r *http.Request) bool {
	allowedMethod := r.Method == http.MethodGet || r.Method == http.MethodHead
	isUpgrade := r.Header.Get("Connection") == "Upgrade" || r.Header.Get("Upgrade") == "websocket"

	return allowedMethod && !isUpgrade
}
