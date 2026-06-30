package core

import (
	"sync"
	"testing"
)

func TestNetworkProxyStore_GetReturnsInitial(t *testing.T) {
	initial := NetworkProxyConfig{Enabled: true, URL: "http://initial:8080"}
	s := NewNetworkProxyStore(initial)
	got := s.Get()
	if !got.Enabled || got.URL != "http://initial:8080" {
		t.Fatalf("Get() = %+v, want initial config", got)
	}
}

func TestNetworkProxyStore_SetReplaces(t *testing.T) {
	s := NewNetworkProxyStore(NetworkProxyConfig{Enabled: false})
	s.Set(NetworkProxyConfig{Enabled: true, URL: "http://new:1080"})
	got := s.Get()
	if !got.Enabled || got.URL != "http://new:1080" {
		t.Fatalf("Get() after Set = %+v, want new config", got)
	}
}

func TestNetworkProxyStore_GetReturnsCopy(t *testing.T) {
	s := NewNetworkProxyStore(NetworkProxyConfig{Enabled: true, URL: "original"})
	got := s.Get()
	got.URL = "mutated" // modify the returned copy
	again := s.Get()
	if again.URL != "original" {
		t.Fatalf("store config mutated via returned copy: got %q, want original", again.URL)
	}
}

func TestNetworkProxyStore_ConcurrentGetSet(t *testing.T) {
	s := NewNetworkProxyStore(NetworkProxyConfig{URL: "init"})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Get()
		}()
		go func(i int) {
			defer wg.Done()
			s.Set(NetworkProxyConfig{URL: "concurrent"})
		}(i)
	}
	wg.Wait()
	// No assertion beyond "no deadlock / no panic". Final state is nondeterministic.
}
