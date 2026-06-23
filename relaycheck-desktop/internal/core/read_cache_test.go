package core

import (
	"testing"
	"time"
)

func TestCachedReadReusesValueUntilInvalidated(t *testing.T) {
	app := &App{readCache: map[string]readCacheEntry{}}
	builds := 0

	first, err := cachedRead(app, "sample", time.Minute, func() (int, error) {
		builds++
		return builds, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := cachedRead(app, "sample", time.Minute, func() (int, error) {
		builds++
		return builds, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if first != 1 || second != 1 || builds != 1 {
		t.Fatalf("expected cached value to be reused, first=%d second=%d builds=%d", first, second, builds)
	}

	app.invalidateReadCache()
	third, err := cachedRead(app, "sample", time.Minute, func() (int, error) {
		builds++
		return builds, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if third != 2 || builds != 2 {
		t.Fatalf("expected invalidation to rebuild, third=%d builds=%d", third, builds)
	}
}
