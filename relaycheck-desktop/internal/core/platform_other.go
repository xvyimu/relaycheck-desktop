//go:build !windows

package core

import (
	"net"
	"syscall"
)

// hiddenProcessAttr returns nil on non-Windows platforms. Used by core code
// (e.g. accounts.go) that spawns child processes.
func hiddenProcessAttr() *syscall.SysProcAttr {
	return nil
}

// netListen creates a TCP listener on the given address. Used by core code
// (e.g. accounts.go) for local browser-login servers.
func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
