package core

import "time"

const (
	shortReadCacheTTL    = 2 * time.Second
	overviewReadCacheTTL = 5 * time.Second
)

type readCacheEntry struct {
	expiresAt time.Time
	value     interface{}
}

func cachedRead[T any](a *App, key string, ttl time.Duration, build func() (T, error)) (T, error) {
	var zero T
	if ttl <= 0 {
		return build()
	}

	nowTime := time.Now()
	a.readCacheMu.RLock()
	if entry, ok := a.readCache[key]; ok && nowTime.Before(entry.expiresAt) {
		if value, ok := entry.value.(T); ok {
			a.readCacheMu.RUnlock()
			return value, nil
		}
	}
	a.readCacheMu.RUnlock()

	value, err := build()
	if err != nil {
		return zero, err
	}

	a.readCacheMu.Lock()
	if a.readCache == nil {
		a.readCache = map[string]readCacheEntry{}
	}
	a.readCache[key] = readCacheEntry{
		expiresAt: time.Now().Add(ttl),
		value:     value,
	}
	a.readCacheMu.Unlock()
	return value, nil
}

func (a *App) invalidateReadCache() {
	a.readCacheMu.Lock()
	a.readCache = map[string]readCacheEntry{}
	a.readCacheMu.Unlock()
}
