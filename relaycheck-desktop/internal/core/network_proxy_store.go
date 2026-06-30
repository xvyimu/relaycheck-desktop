package core

import "sync"

// NetworkProxyStore holds the current network proxy configuration. It owns
// its own RWMutex, independent of App's global lock.
type NetworkProxyStore struct {
	mu     sync.RWMutex
	config NetworkProxyConfig
}

func NewNetworkProxyStore(initial NetworkProxyConfig) *NetworkProxyStore {
	return &NetworkProxyStore{config: initial}
}

// Get returns a snapshot of the current proxy config.
func (s *NetworkProxyStore) Get() NetworkProxyConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Set replaces the current proxy config.
func (s *NetworkProxyStore) Set(cfg NetworkProxyConfig) {
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
}
