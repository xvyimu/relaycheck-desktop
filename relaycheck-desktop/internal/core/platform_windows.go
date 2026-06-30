//go:build windows

package core

import (
	"net"
	"syscall"
)

// hiddenProcessAttr returns a SysProcAttr that hides the child window. Used
// by core code (e.g. accounts.go) that spawns child processes without a
// console window. The autostart domain has its own copy.
func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

// netListen creates a TCP listener on the given address. Used by core code
// (e.g. accounts.go) for local browser-login servers.
func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
