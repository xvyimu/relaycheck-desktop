package core

import "sync"

// SyncJobRunStore tracks the running state of scheduled sync jobs (local
// NewAPI sync, channel health probe) to prevent re-entrant execution. Each
// store owns its own mutex, independent of App's global lock.
type SyncJobRunStore struct {
	mu      sync.Mutex
	running bool
}

// NewSyncJobRunStore creates an empty SyncJobRunStore.
func NewSyncJobRunStore() *SyncJobRunStore {
	return &SyncJobRunStore{}
}

// TryStart attempts to mark the job as running. Returns false if already
// running. The caller must call Finish on the same store when done, even on
// error paths.
func (s *SyncJobRunStore) TryStart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

// Finish marks the job as no longer running.
func (s *SyncJobRunStore) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}
