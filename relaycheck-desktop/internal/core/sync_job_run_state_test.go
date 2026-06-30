package core

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSyncJobRunStore_TryStartReturnsTrueWhenIdle(t *testing.T) {
	s := NewSyncJobRunStore()
	if !s.TryStart() {
		t.Fatal("TryStart on idle store should return true")
	}
}

func TestSyncJobRunStore_TryStartReturnsFalseWhenRunning(t *testing.T) {
	s := NewSyncJobRunStore()
	if !s.TryStart() {
		t.Fatal("first TryStart should succeed")
	}
	if s.TryStart() {
		t.Fatal("second TryStart while running should return false")
	}
}

func TestSyncJobRunStore_FinishAllowsRestart(t *testing.T) {
	s := NewSyncJobRunStore()
	if !s.TryStart() {
		t.Fatal("first TryStart should succeed")
	}
	s.Finish()
	if !s.TryStart() {
		t.Fatal("TryStart after Finish should succeed")
	}
}

func TestSyncJobRunStore_ConcurrentTryStartOnlyOneSucceeds(t *testing.T) {
	s := NewSyncJobRunStore()
	var successCount int64
	var wg sync.WaitGroup
	goroutines := 100
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if s.TryStart() {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successCount != 1 {
		t.Fatalf("concurrent TryStart success count = %d, want exactly 1", successCount)
	}
}

func TestSyncJobRunStore_FinishWithoutStartIsNoop(t *testing.T) {
	s := NewSyncJobRunStore()
	// Calling Finish on a freshly-created store must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Finish on idle store panicked: %v", r)
		}
	}()
	s.Finish()
	// After a no-op Finish, TryStart should still work.
	if !s.TryStart() {
		t.Fatal("TryStart after no-op Finish should succeed")
	}
}

func TestSyncJobRunStore_RepeatedFinishIsNoop(t *testing.T) {
	s := NewSyncJobRunStore()
	s.TryStart()
	s.Finish()
	s.Finish()
	s.Finish()
	// After multiple Finish, TryStart should still succeed (running is false).
	if !s.TryStart() {
		t.Fatal("TryStart after repeated Finish should succeed")
	}
}
