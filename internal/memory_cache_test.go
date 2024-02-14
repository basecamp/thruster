package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryCache_store_and_retrieve(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	c.Set(1, []byte("hello world"), time.Now().Add(30*time.Second))

	read, ok := c.Get(1)
	assert.True(t, ok)
	assert.Equal(t, []byte("hello world"), read)
}

func TestMemoryCache_storing_updates_existing_value(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	c.Set(1, []byte("first"), time.Now().Add(30*time.Second))
	c.Set(1, []byte("second"), time.Now().Add(30*time.Second))

	read, ok := c.Get(1)
	assert.True(t, ok)
	assert.Equal(t, []byte("second"), read)
}

func TestMemoryCache_expiry(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	now := time.Date(2023, 1, 22, 17, 30, 0, 0, time.UTC)

	c.getCurrentTime = func() time.Time { return now }
	c.Set(1, []byte("hello world"), now.Add(1*time.Second))

	read, ok := c.Get(1)
	assert.True(t, ok)
	assert.Equal(t, []byte("hello world"), read)

	c.getCurrentTime = func() time.Time { return now.Add(2 * time.Second) }

	_, ok = c.Get(1)
	assert.False(t, ok)
}

func TestMemoryCache_does_not_store_items_over_cache_limit(t *testing.T) {
	c := NewMemoryCache(3*KB, 50*KB)

	payload := make([]byte, 10*KB)
	c.Set(1, payload, time.Now().Add(1*time.Hour))

	_, ok := c.Get(1)
	assert.False(t, ok)
}

func TestMemoryCache_does_not_store_items_over_item_limit(t *testing.T) {
	c := NewMemoryCache(50*KB, 3*KB)

	payload := make([]byte, 10*KB)
	c.Set(1, payload, time.Now().Add(1*time.Hour))

	_, ok := c.Get(1)
	assert.False(t, ok)
}

func BenchmarkCache_populating_small_objects(b *testing.B) {
	c := NewMemoryCache(32*MB, 1*MB)
	payload := make([]byte, KB)
	expires := time.Now().Add(1 * time.Hour)

	for i := 0; i < b.N; i++ {
		c.Set(uint64(i), payload, expires)
		c.Get(uint64(i))
	}
}

func BenchmarkCache_populating_large_objects(b *testing.B) {
	c := NewMemoryCache(32*MB, 1*MB)
	payload := make([]byte, 512*KB)
	expires := time.Now().Add(1 * time.Hour)

	for i := 0; i < b.N; i++ {
		c.Set(uint64(i), payload, expires)
		c.Get(uint64(i))
	}
}
