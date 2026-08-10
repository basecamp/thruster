package internal

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testKey(i int) RequestKey {
	return RequestKey{Method: "GET", Path: "/" + strconv.Itoa(i)}
}

func TestMemoryCache_store_and_retrieve(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	c.Set(testKey(1), []byte("hello world"), time.Now().Add(30*time.Second))

	read, ok := c.Get(testKey(1))
	assert.True(t, ok)
	assert.Equal(t, []byte("hello world"), read)
}

func TestMemoryCache_storing_updates_existing_value(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	c.Set(testKey(1), []byte("first"), time.Now().Add(30*time.Second))
	c.Set(testKey(1), []byte("second"), time.Now().Add(30*time.Second))

	read, ok := c.Get(testKey(1))
	assert.True(t, ok)
	assert.Equal(t, []byte("second"), read)
}

func TestMemoryCache_storing_existing_value_keeps_keys_and_size_correct(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	c.Set(testKey(1), []byte("first"), time.Now().Add(30*time.Second))
	c.Set(testKey(1), []byte("second"), time.Now().Add(30*time.Second))

	assert.Equal(t, 1, len(c.keys))
	assert.Equal(t, testKey(1).size()+6, c.size)
}

func TestMemoryCache_expiry(t *testing.T) {
	c := NewMemoryCache(32*MB, 1*MB)
	now := time.Date(2023, 1, 22, 17, 30, 0, 0, time.UTC)

	c.getCurrentTime = func() time.Time { return now }
	c.Set(testKey(1), []byte("hello world"), now.Add(1*time.Second))

	read, ok := c.Get(testKey(1))
	assert.True(t, ok)
	assert.Equal(t, []byte("hello world"), read)

	c.getCurrentTime = func() time.Time { return now.Add(2 * time.Second) }

	_, ok = c.Get(testKey(1))
	assert.False(t, ok)
}

func TestMemoryCache_does_not_store_items_over_cache_limit(t *testing.T) {
	c := NewMemoryCache(3*KB, 50*KB)

	payload := make([]byte, 10*KB)
	c.Set(testKey(1), payload, time.Now().Add(1*time.Hour))

	_, ok := c.Get(testKey(1))
	assert.False(t, ok)
}

func TestMemoryCache_of_size_zero_does_not_store_items(t *testing.T) {
	c := NewMemoryCache(0, 1*KB)

	c.Set(testKey(1), []byte("There's nowhere to store this"), time.Now().Add(1*time.Hour))

	_, ok := c.Get(testKey(1))
	assert.False(t, ok)
}

func TestMemoryCache_items_are_evicted_to_make_space(t *testing.T) {
	maxCacheSize := 10 * KB
	c := NewMemoryCache(maxCacheSize, 1*KB)

	for i := range 20 {
		payload := bytes.Repeat([]byte{byte(i)}, 1*KB)
		c.Set(testKey(i), payload, time.Now().Add(1*time.Hour))

		retrieved, ok := c.Get(testKey(i))
		assert.True(t, ok)
		assert.Equal(t, payload, retrieved)
	}

	expectedSize := 0
	for key, item := range c.items {
		expectedSize += key.size() + len(item.value)
	}

	assert.Equal(t, expectedSize, c.size)
	assert.LessOrEqual(t, c.size, maxCacheSize)
	assert.Equal(t, len(c.keys), len(c.items))
	assert.Greater(t, len(c.items), 0)
}

func TestMemoryCache_does_not_store_items_over_item_limit(t *testing.T) {
	c := NewMemoryCache(50*KB, 3*KB)

	payload := make([]byte, 10*KB)
	c.Set(testKey(1), payload, time.Now().Add(1*time.Hour))

	_, ok := c.Get(testKey(1))
	assert.False(t, ok)
}

func BenchmarkCache_populating_small_objects(b *testing.B) {
	c := NewMemoryCache(32*MB, 1*MB)
	payload := make([]byte, KB)
	expires := time.Now().Add(1 * time.Hour)

	for i := 0; i < b.N; i++ {
		c.Set(testKey(i), payload, expires)
		c.Get(testKey(i))
	}
}

func BenchmarkCache_populating_large_objects(b *testing.B) {
	c := NewMemoryCache(32*MB, 1*MB)
	payload := make([]byte, 512*KB)
	expires := time.Now().Add(1 * time.Hour)

	for i := 0; i < b.N; i++ {
		c.Set(testKey(i), payload, expires)
		c.Get(testKey(i))
	}
}
