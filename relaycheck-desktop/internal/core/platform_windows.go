//go:build windows

package core

import (
	"net"
	"syscall"
)

func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
