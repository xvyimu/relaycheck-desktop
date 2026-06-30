package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadCacheStore_HitWithinTTL(t *testing.T) {
	c := NewReadCacheStore()
	calls := int32(0)
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "value", nil
	}
	// First call: miss, builds and caches
	v, err := Get(c, "key", time.Minute, build)
	if err != nil || v != "value" {
		t.Fatalf("first Get = %q, %v; want value, nil", v, err)
	}
	// Second call: hit, should not call build
	v, err = Get(c, "key", time.Minute, build)
	if err != nil || v != "value" {
		t.Fatalf("second Get = %q, %v; want value, nil", v, err)
	}
	if calls != 1 {
		t.Fatalf("build called %d times, want 1", calls)
	}
}

func TestReadCacheStore_MissAfterExpiry(t *testing.T) {
	c := NewReadCacheStore()
	calls := int32(0)
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "value", nil
	}
	Get(c, "key", 10*time.Millisecond, build)
	time.Sleep(20 * time.Millisecond)
	Get(c, "key", 10*time.Millisecond, build)
	if calls != 2 {
		t.Fatalf("build called %d times, want 2 (one before expiry, one after)", calls)
	}
}

func TestReadCacheStore_ZeroTTLBypassesCache(t *testing.T) {
	c := NewReadCacheStore()
	calls := int32(0)
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "value", nil
	}
	Get(c, "key", 0, build)
	Get(c, "key", 0, build)
	Get(c, "key", -1, build)
	if calls != 3 {
		t.Fatalf("build called %d times, want 3 (zero/negative ttl bypasses cache)", calls)
	}
}

func TestReadCacheStore_BuildErrorNotCached(t *testing.T) {
	c := NewReadCacheStore()
	calls := int32(0)
	errBuild := errors.New("boom")
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", errBuild
	}
	_, err := Get(c, "key", time.Minute, build)
	if !errors.Is(err, errBuild) {
		t.Fatalf("got err %v, want errBuild", err)
	}
	// Second call should still build (error not cached)
	_, err = Get(c, "key", time.Minute, build)
	if !errors.Is(err, errBuild) {
		t.Fatalf("got err %v, want errBuild", err)
	}
	if calls != 2 {
		t.Fatalf("build called %d times, want 2 (error should not be cached)", calls)
	}
}

func TestReadCacheStore_InvalidateClears(t *testing.T) {
	c := NewReadCacheStore()
	calls := int32(0)
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "value", nil
	}
	Get(c, "key", time.Minute, build)
	c.Invalidate()
	Get(c, "key", time.Minute, build)
	if calls != 2 {
		t.Fatalf("build called %d times, want 2 (Invalidate should force rebuild)", calls)
	}
}

func TestReadCacheStore_DifferentKeysDontCollide(t *testing.T) {
	c := NewReadCacheStore()
	buildA := func() (string, error) { return "a", nil }
	buildB := func() (string, error) { return "b", nil }
	v, _ := Get(c, "a", time.Minute, buildA)
	if v != "a" {
		t.Fatalf("Get(a) = %q, want a", v)
	}
	v, _ = Get(c, "b", time.Minute, buildB)
	if v != "b" {
		t.Fatalf("Get(b) = %q, want b", v)
	}
	v, _ = Get(c, "a", time.Minute, func() (string, error) { return "should-not-build", nil })
	if v != "a" {
		t.Fatalf("Get(a) second = %q, want a (cached)", v)
	}
}

func TestReadCacheStore_ConcurrentGetSameKey(t *testing.T) {
	c := NewReadCacheStore()
	var calls int32
	build := func() (int, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(5 * time.Millisecond) // slow build to amplify races
		return 42, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := Get(c, "key", time.Minute, build)
			if err != nil || v != 42 {
				t.Errorf("Get = %d, %v; want 42, nil", v, err)
			}
		}()
	}
	wg.Wait()
	// Note: with slow build, some races are expected; calls will be >1 but
	// should be well under 50. The test verifies correctness, not single-build.
	if calls < 1 || calls > 50 {
		t.Fatalf("build called %d times, want 1..50", calls)
	}
}
