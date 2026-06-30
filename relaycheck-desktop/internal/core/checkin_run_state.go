package core

import (
	"fmt"
	"sync"
)

// CheckinRunStore owns the checkin run state together with its own mutex,
// decoupling it from the App global lock (a.mu).
type CheckinRunStore struct {
	mu    sync.RWMutex
	state checkinRunState
}

// NewCheckinRunStore creates an empty CheckinRunStore.
func NewCheckinRunStore() *CheckinRunStore {
	return &CheckinRunStore{}
}

// begin starts a new checkin run. It returns false if a run is already in
// progress. The semantics match the original App.beginCheckinRun.
func (s *CheckinRunStore) begin(mode string, total int) bool {
	timestamp := now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Running {
		return false
	}
	s.state = checkinRunState{
		Running:       true,
		Mode:          mode,
		TotalAccounts: total,
		StartedAt:     timestamp,
		UpdatedAt:     timestamp,
	}
	if total == 0 {
		s.state.CurrentMessage = "今天没有待签到账号。"
	}
	return true
}

// updateCurrent records the account currently being processed.
func (s *CheckinRunStore) updateCurrent(accountID string, accountName string, siteName string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Running {
		return
	}
	s.state.CurrentAccountID = accountID
	s.state.CurrentAccount = accountName
	s.state.CurrentSite = siteName
	s.state.CurrentMessage = message
	s.state.UpdatedAt = now()
}

// updateMessage sets both the current and last run message.
func (s *CheckinRunStore) updateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CurrentMessage = message
	s.state.LastRunMessage = message
	s.state.UpdatedAt = now()
}

// recordResult increments the appropriate counter based on status and updates
// the current/last run message.
func (s *CheckinRunStore) recordResult(status string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Running {
		return
	}
	s.state.ProcessedAccounts++
	s.state.CurrentMessage = firstNonEmpty(message, status)
	s.state.LastRunMessage = s.state.CurrentMessage
	switch status {
	case "success":
		s.state.SuccessCount++
	case "already_checked":
		s.state.AlreadyCount++
	case "unsupported":
		s.state.UnsupportedCount++
	case "auth_expired", "manual_required":
		s.state.AuthExpiredCount++
	default:
		s.state.FailedCount++
	}
	s.state.UpdatedAt = now()
}

// finish marks the run as finished and clears the current account fields.
func (s *CheckinRunStore) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	timestamp := now()
	s.state.Running = false
	s.state.FinishedAt = timestamp
	s.state.UpdatedAt = timestamp
	s.state.CurrentAccountID = ""
	s.state.CurrentAccount = ""
	s.state.CurrentSite = ""
	if s.state.LastRunMessage == "" {
		s.state.LastRunMessage = fmt.Sprintf("本轮处理 %d 个账号。", s.state.ProcessedAccounts)
	}
}

// Snapshot returns a copy of the current state under a read lock.
func (s *CheckinRunStore) Snapshot() checkinRunState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}
