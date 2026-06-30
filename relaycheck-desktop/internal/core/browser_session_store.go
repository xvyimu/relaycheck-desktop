package core

import "sync"

// BrowserSessionStore manages Chrome browser login sessions. It owns its
// own mutex, independent of App's global lock, so session operations do
// not contend with other App state.
type BrowserSessionStore struct {
	mu       sync.Mutex
	sessions map[string]BrowserLoginSession
}

func NewBrowserSessionStore() *BrowserSessionStore {
	return &BrowserSessionStore{sessions: map[string]BrowserLoginSession{}}
}

// Get returns the session for the given account ID and whether it exists.
func (s *BrowserSessionStore) Get(id string) (BrowserLoginSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	return session, ok
}

// Set stores a session for the given account ID.
func (s *BrowserSessionStore) Set(id string, session BrowserLoginSession) {
	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()
}

// Delete removes the session for the given account ID.
func (s *BrowserSessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// DeleteIfPIDMatches removes the session for id only if its PID matches
// the given pid. Used by the Chrome watchdog goroutine to avoid deleting
// a newer session that replaced the one it was watching.
func (s *BrowserSessionStore) DeleteIfPIDMatches(id string, pid int) {
	s.mu.Lock()
	if existing, ok := s.sessions[id]; ok && existing.PID == pid {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
}

// List returns a snapshot of all sessions as a slice.
func (s *BrowserSessionStore) List() []BrowserLoginSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]BrowserLoginSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		result = append(result, session)
	}
	return result
}

// Len returns the number of active sessions.
func (s *BrowserSessionStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// Range calls fn for each session. It holds the lock during iteration,
// so fn must not call back into the store (deadlock). Used for the
// bulk-finish iteration that just reads session fields.
func (s *BrowserSessionStore) Range(fn func(id string, session BrowserLoginSession)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		fn(id, session)
	}
}
