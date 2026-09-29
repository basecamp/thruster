package internal

import (
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

type GetCurrentTime func() time.Time

type MemoryCacheEntry struct {
	lastAccessedAt time.Time
	expiresAt      time.Time
	value          []byte
}

type MemoryCacheEntryMap map[RequestKey]*MemoryCacheEntry
type MemoryCacheKeyList []RequestKey

type MemoryCache struct {
	sync.Mutex
	capacity       int
	maxItemSize    int
	size           int
	keys           MemoryCacheKeyList
	items          MemoryCacheEntryMap
	getCurrentTime GetCurrentTime
}

func NewMemoryCache(capacity, maxItemSize int) *MemoryCache {
	return &MemoryCache{
		capacity:       capacity,
		maxItemSize:    maxItemSize,
		size:           0,
		keys:           MemoryCacheKeyList{},
		items:          MemoryCacheEntryMap{},
		getCurrentTime: time.Now,
	}
}

func (c *MemoryCache) Set(key RequestKey, value []byte, expiresAt time.Time) {
	c.Lock()
	defer c.Unlock()

	itemSize := key.size() + len(value)
	if len(value) > c.maxItemSize || itemSize > c.capacity {
		slog.Debug("Cache: item is too large to store", "len", itemSize)
		return
	}

	limit := c.capacity - itemSize
	for c.size > limit {
		slog.Debug("Cache: evicting item to make space", "current_size", c.size, "need_size", limit)
		c.evictOldestItem()
	}

	existingItem, ok := c.items[key]
	if ok {
		c.size -= key.size() + len(existingItem.value)
	} else {
		c.keys = append(c.keys, key)
	}

	c.items[key] = &MemoryCacheEntry{
		lastAccessedAt: c.getCurrentTime(),
		expiresAt:      expiresAt,
		value:          value,
	}

	c.size += itemSize

	slog.Debug("Cache: added item", "size", itemSize, "expires_at", expiresAt)
}

func (c *MemoryCache) Get(key RequestKey) ([]byte, bool) {
	c.Lock()
	defer c.Unlock()

	now := c.getCurrentTime()

	item, ok := c.items[key]
	if !ok || item.expiresAt.Before(now) {
		return nil, false
	}

	item.lastAccessedAt = now
	return item.value, true
}

func (c *MemoryCache) evictOldestItem() {
	var oldestKey RequestKey
	var oldestIndex int
	var oldest time.Time

	now := c.getCurrentTime()

	// Pick 5 random items and evict the oldest one, On average we'll evict items
	// in the oldest 20%, which is good enough and is much faster than scanning
	// through them all.
	//
	// If we find an expired item while looking, that's a better choice to evict,
	// so we can choose it immediately.
	for range 5 {
		index := rand.Intn(len(c.keys))
		key := c.keys[index]
		v := c.items[key]

		if v.expiresAt.Before(now) {
			oldestKey = key
			oldestIndex = index
			break
		}

		if v.lastAccessedAt.Before(oldest) || oldest.IsZero() {
			oldest = v.lastAccessedAt
			oldestKey = key
			oldestIndex = index
		}
	}

	c.keys[oldestIndex] = c.keys[len(c.keys)-1]
	c.keys = c.keys[:len(c.keys)-1]

	c.size -= oldestKey.size() + len(c.items[oldestKey].value)
	delete(c.items, oldestKey)
}
