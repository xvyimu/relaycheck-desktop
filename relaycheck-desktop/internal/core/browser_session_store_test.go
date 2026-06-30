package core

import (
	"sync"
	"testing"
	"time"
)

func TestBrowserSessionStore_SetAndGet(t *testing.T) {
	s := NewBrowserSessionStore()
	session := BrowserLoginSession{
		AccountID: "acc1",
		Port:      9222,
		StartedAt: time.Now(),
		PID:       100,
	}
	s.Set("acc1", session)
	got, ok := s.Get("acc1")
	if !ok {
		t.Fatal("Get after Set returned ok=false, want true")
	}
	if got.PID != 100 {
		t.Fatalf("PID = %d, want 100", got.PID)
	}
	if got.AccountID != "acc1" {
		t.Fatalf("AccountID = %q, want %q", got.AccountID, "acc1")
	}
	if got.Port != 9222 {
		t.Fatalf("Port = %d, want 9222", got.Port)
	}
}

func TestBrowserSessionStore_GetMissingReturnsFalse(t *testing.T) {
	s := NewBrowserSessionStore()
	got, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("Get on missing id returned ok=true, want false")
	}
	if got.PID != 0 || got.AccountID != "" {
		t.Fatalf("zero-value expected on missing Get, got %+v", got)
	}
}

func TestBrowserSessionStore_Delete(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 100})
	s.Delete("acc1")
	if _, ok := s.Get("acc1"); ok {
		t.Fatal("Get after Delete returned ok=true, want false")
	}
}

func TestBrowserSessionStore_DeleteMissingIsNoop(t *testing.T) {
	s := NewBrowserSessionStore()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Delete on missing id panicked: %v", r)
		}
	}()
	s.Delete("nonexistent")
}

func TestBrowserSessionStore_DeleteIfPIDMatches_Success(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 100})
	s.DeleteIfPIDMatches("acc1", 100)
	if _, ok := s.Get("acc1"); ok {
		t.Fatal("Get after DeleteIfPIDMatches with matching PID returned ok=true, want false")
	}
}

func TestBrowserSessionStore_DeleteIfPIDMatches_PIDMismatchKeepsSession(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 100})
	// Watchdog semantics: PID mismatch means the session was replaced by a
	// newer one; the watchdog must NOT delete it.
	s.DeleteIfPIDMatches("acc1", 999)
	got, ok := s.Get("acc1")
	if !ok {
		t.Fatal("Get after DeleteIfPIDMatches with mismatched PID returned ok=false, want true")
	}
	if got.PID != 100 {
		t.Fatalf("PID = %d, want 100 (session should be unchanged)", got.PID)
	}
}

func TestBrowserSessionStore_DeleteIfPIDMatches_MissingIdIsNoop(t *testing.T) {
	s := NewBrowserSessionStore()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DeleteIfPIDMatches on missing id panicked: %v", r)
		}
	}()
	s.DeleteIfPIDMatches("nonexistent", 100)
}

func TestBrowserSessionStore_ListReturnsAllSessions(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 1})
	s.Set("acc2", BrowserLoginSession{PID: 2})
	s.Set("acc3", BrowserLoginSession{PID: 3})
	list := s.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
	// List must return a snapshot; mutating it must not affect the store.
	list[0].PID = 9999
	if got, _ := s.Get("acc1"); got.PID == 9999 {
		t.Fatal("mutating List result affected the store (PID changed)")
	}
}

func TestBrowserSessionStore_Len(t *testing.T) {
	s := NewBrowserSessionStore()
	if s.Len() != 0 {
		t.Fatalf("Len on empty store = %d, want 0", s.Len())
	}
	s.Set("acc1", BrowserLoginSession{PID: 1})
	s.Set("acc2", BrowserLoginSession{PID: 2})
	if s.Len() != 2 {
		t.Fatalf("Len after 2 Sets = %d, want 2", s.Len())
	}
	s.Delete("acc1")
	if s.Len() != 1 {
		t.Fatalf("Len after Delete = %d, want 1", s.Len())
	}
}

func TestBrowserSessionStore_RangeIteratesAll(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 1})
	s.Set("acc2", BrowserLoginSession{PID: 2})
	s.Set("acc3", BrowserLoginSession{PID: 3})
	count := 0
	pids := map[int]bool{}
	s.Range(func(id string, session BrowserLoginSession) {
		count++
		pids[session.PID] = true
	})
	if count != 3 {
		t.Fatalf("Range callback invoked %d times, want 3", count)
	}
	if !pids[1] || !pids[2] || !pids[3] {
		t.Fatalf("Range did not cover all PIDs: %v", pids)
	}
}

// TestBrowserSessionStore_RangeCallbackMustNotMutate documents the lock
// semantics of Range: Range holds the store mutex for the full iteration,
// so the callback MUST NOT call back into the store (Set/Delete/Get/Len/...)
// because sync.Mutex is non-reentrant and doing so would deadlock.
//
// We cannot assert the deadlock at runtime in a portable way (it would hang
// the test runner), so this test only verifies that a read-only callback
// works correctly and observes a consistent snapshot. The prohibition on
// mutation from within the callback is a documented contract, enforced by
// code review at the call sites (the only production caller is the
// bulk-finish iteration, which only reads session fields).
func TestBrowserSessionStore_RangeCallbackMustNotMutate(t *testing.T) {
	s := NewBrowserSessionStore()
	s.Set("acc1", BrowserLoginSession{PID: 1})
	s.Set("acc2", BrowserLoginSession{PID: 2})

	seen := make(map[string]int)
	s.Range(func(id string, session BrowserLoginSession) {
		// Read-only access is safe.
		seen[id] = session.PID
	})

	if len(seen) != 2 {
		t.Fatalf("Range saw %d entries, want 2", len(seen))
	}
	// Mutations after Range returns are safe.
	s.Set("acc3", BrowserLoginSession{PID: 3})
	if s.Len() != 3 {
		t.Fatalf("Len after post-Range Set = %d, want 3", s.Len())
	}
}

func TestBrowserSessionStore_ConcurrentSetGet(t *testing.T) {
	// Smoke test that concurrent Set/Get/Delete do not race. Run with -race
	// (where available) to surface data races; on this Windows environment
	// cgo/-race is disabled, so this mainly guards against deadlocks and
	// panics under contention.
	s := NewBrowserSessionStore()
	var wg sync.WaitGroup
	writers := 20
	readers := 20
	deleters := 5
	wg.Add(writers + readers + deleters)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		go func(n int) {
			defer wg.Done()
			<-start
			id := "acc" + itoa(n)
			s.Set(id, BrowserLoginSession{PID: n})
		}(i)
	}
	for i := 0; i < readers; i++ {
		go func(n int) {
			defer wg.Done()
			<-start
			_, _ = s.Get("acc" + itoa(n%writers))
		}(i)
	}
	for i := 0; i < deleters; i++ {
		go func(n int) {
			defer wg.Done()
			<-start
			s.Delete("acc" + itoa(n%writers))
		}(i)
	}
	close(start)
	wg.Wait()
}

// itoa is a tiny dependency-free int->string helper to avoid importing fmt
// in the concurrent test hot path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
