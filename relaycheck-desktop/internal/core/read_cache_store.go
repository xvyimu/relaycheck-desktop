package core

import (
	"sync"
	"time"
)

const (
	shortReadCacheTTL    = 2 * time.Second
	overviewReadCacheTTL = 5 * time.Second
)

type readCacheEntry struct {
	expiresAt time.Time
	value     interface{}
}

// ReadCacheStore holds the short-lived read cache for dashboard/overview
// queries. It has its own RWMutex, independent of App's global lock, so
// cache reads/writes do not contend with other App state.
type ReadCacheStore struct {
	mu    sync.RWMutex
	store map[string]readCacheEntry
}

func NewReadCacheStore() *ReadCacheStore {
	return &ReadCacheStore{store: map[string]readCacheEntry{}}
}

// Get returns the cached value for key if it exists and has not expired,
// otherwise it calls build to compute the value, caches it with the given
// ttl, and returns it. A ttl <= 0 bypasses the cache entirely.
func Get[T any](c *ReadCacheStore, key string, ttl time.Duration, build func() (T, error)) (T, error) {
	var zero T
	if ttl <= 0 {
		return build()
	}

	nowTime := time.Now()
	c.mu.RLock()
	if entry, ok := c.store[key]; ok && nowTime.Before(entry.expiresAt) {
		if value, ok := entry.value.(T); ok {
			c.mu.RUnlock()
			return value, nil
		}
	}
	c.mu.RUnlock()

	value, err := build()
	if err != nil {
		return zero, err
	}

	c.mu.Lock()
	if c.store == nil {
		c.store = map[string]readCacheEntry{}
	}
	c.store[key] = readCacheEntry{
		expiresAt: time.Now().Add(ttl),
		value:     value,
	}
	c.mu.Unlock()
	return value, nil
}

// Invalidate clears all cached entries.
func (c *ReadCacheStore) Invalidate() {
	c.mu.Lock()
	c.store = map[string]readCacheEntry{}
	c.mu.Unlock()
}

// cachedRead is a thin forwarding helper so existing call sites
// (accounts, channels, channel_health, models_pricing, routes, sites,
// usage_overview, read_cache_test) need no changes. New code should call
// Get(a.readCache, ...) directly.
func cachedRead[T any](a *App, key string, ttl time.Duration, build func() (T, error)) (T, error) {
	return Get(a.readCache, key, ttl, build)
}

// invalidateReadCache forwards to the store. Kept as an *App method so the
// 8 existing call sites remain unchanged.
func (a *App) invalidateReadCache() {
	a.readCache.Invalidate()
}
