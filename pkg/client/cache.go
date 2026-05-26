package client

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

type ResponseCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

func NewResponseCache(ttl time.Duration) *ResponseCache {
	return &ResponseCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func (c *ResponseCache) GetOrFetch(key string, fetch func() ([]byte, error)) ([]byte, error) {
	if c == nil {
		return fetch()
	}
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && time.Now().Before(e.expiresAt) {
		c.mu.Unlock()
		return e.data, nil
	}
	c.mu.Unlock()

	data, err := fetch()
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return data, nil
}
