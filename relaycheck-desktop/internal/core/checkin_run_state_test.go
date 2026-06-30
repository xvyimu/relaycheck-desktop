package core

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestCheckinRunStore_BeginReturnsTrueWhenIdle(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 3) {
		t.Fatal("begin on idle store should return true")
	}
	snap := s.Snapshot()
	if !snap.Running {
		t.Fatalf("after begin, state.Running = false, want true")
	}
	if snap.Mode != "manual" {
		t.Fatalf("Mode = %q, want %q", snap.Mode, "manual")
	}
	if snap.TotalAccounts != 3 {
		t.Fatalf("TotalAccounts = %d, want 3", snap.TotalAccounts)
	}
	if snap.StartedAt == "" {
		t.Fatal("StartedAt should be set after begin")
	}
}

func TestCheckinRunStore_BeginReturnsFalseWhenAlreadyRunning(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 3) {
		t.Fatal("first begin should succeed")
	}
	if s.begin("auto", 5) {
		t.Fatal("second begin while running should return false")
	}
	// Mode/Total must remain from the first run.
	snap := s.Snapshot()
	if snap.Mode != "manual" || snap.TotalAccounts != 3 {
		t.Fatalf("state mutated by rejected begin: Mode=%q Total=%d", snap.Mode, snap.TotalAccounts)
	}
}

func TestCheckinRunStore_FinishResetsRunning(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 2) {
		t.Fatal("begin should succeed")
	}
	s.finish()
	snap := s.Snapshot()
	if snap.Running {
		t.Fatal("after finish, Running = true, want false")
	}
	if snap.FinishedAt == "" {
		t.Fatal("FinishedAt should be set after finish")
	}
	// After finish, begin must work again.
	if !s.begin("auto", 4) {
		t.Fatal("begin after finish should succeed")
	}
}

func TestCheckinRunStore_UpdateCurrentSetsFields(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 1) {
		t.Fatal("begin should succeed")
	}
	s.updateCurrent("acc1", "Account1", "Site1", "处理中")
	snap := s.Snapshot()
	if snap.CurrentAccountID != "acc1" {
		t.Fatalf("CurrentAccountID = %q, want %q", snap.CurrentAccountID, "acc1")
	}
	if snap.CurrentAccount != "Account1" {
		t.Fatalf("CurrentAccount = %q, want %q", snap.CurrentAccount, "Account1")
	}
	if snap.CurrentSite != "Site1" {
		t.Fatalf("CurrentSite = %q, want %q", snap.CurrentSite, "Site1")
	}
	if snap.CurrentMessage != "处理中" {
		t.Fatalf("CurrentMessage = %q, want %q", snap.CurrentMessage, "处理中")
	}
}

func TestCheckinRunStore_UpdateCurrentBeforeBeginIsNoop(t *testing.T) {
	s := NewCheckinRunStore()
	// Without begin, updateCurrent must not populate fields.
	s.updateCurrent("acc1", "Account1", "Site1", "处理中")
	snap := s.Snapshot()
	if snap.CurrentAccountID != "" || snap.CurrentAccount != "" || snap.CurrentSite != "" {
		t.Fatalf("updateCurrent before begin mutated state: %+v", snap)
	}
}

func TestCheckinRunStore_RecordResultIncrementsCounters(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 3) {
		t.Fatal("begin should succeed")
	}
	s.recordResult("success", "ok")
	s.recordResult("success", "ok2")
	s.recordResult("failed", "err")
	snap := s.Snapshot()
	if snap.ProcessedAccounts != 3 {
		t.Fatalf("ProcessedAccounts = %d, want 3", snap.ProcessedAccounts)
	}
	if snap.SuccessCount != 2 {
		t.Fatalf("SuccessCount = %d, want 2", snap.SuccessCount)
	}
	if snap.FailedCount != 1 {
		t.Fatalf("FailedCount = %d, want 1", snap.FailedCount)
	}
	if snap.LastRunMessage != "err" {
		t.Fatalf("LastRunMessage = %q, want %q", snap.LastRunMessage, "err")
	}
}

func TestCheckinRunStore_RecordResultBeforeBeginIsNoop(t *testing.T) {
	s := NewCheckinRunStore()
	s.recordResult("success", "ok")
	snap := s.Snapshot()
	if snap.ProcessedAccounts != 0 || snap.SuccessCount != 0 {
		t.Fatalf("recordResult before begin mutated state: %+v", snap)
	}
}

func TestCheckinRunStore_SnapshotReturnsCopy(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 2) {
		t.Fatal("begin should succeed")
	}
	snap1 := s.Snapshot()
	// Mutate the returned copy.
	snap1.Running = false
	snap1.Mode = "tampered"
	snap1.CurrentAccountID = "hacked"
	snap1.SuccessCount = 999
	// Re-snapshot; original state must be unaffected.
	snap2 := s.Snapshot()
	if !snap2.Running {
		t.Fatal("mutating snapshot affected store: Running = false")
	}
	if snap2.Mode != "manual" {
		t.Fatalf("Mode = %q, want %q", snap2.Mode, "manual")
	}
	if snap2.CurrentAccountID != "" {
		t.Fatalf("CurrentAccountID = %q, want empty", snap2.CurrentAccountID)
	}
	if snap2.SuccessCount != 0 {
		t.Fatalf("SuccessCount = %d, want 0", snap2.SuccessCount)
	}
}

func TestCheckinRunStore_ConcurrentBeginOnlyOneSucceeds(t *testing.T) {
	s := NewCheckinRunStore()
	var successCount int64
	var wg sync.WaitGroup
	goroutines := 100
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if s.begin("concurrent", 1) {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successCount != 1 {
		t.Fatalf("concurrent begin success count = %d, want exactly 1", successCount)
	}
}

func TestCheckinRunStore_BeginWithZeroTotalSetsEmptyMessage(t *testing.T) {
	s := NewCheckinRunStore()
	if !s.begin("manual", 0) {
		t.Fatal("begin should succeed")
	}
	snap := s.Snapshot()
	if snap.TotalAccounts != 0 {
		t.Fatalf("TotalAccounts = %d, want 0", snap.TotalAccounts)
	}
	if snap.CurrentMessage == "" {
		t.Fatal("CurrentMessage should be populated when total is 0")
	}
}
