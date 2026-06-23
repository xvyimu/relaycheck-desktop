//go:build !windows

package core

import (
	"net"
	"syscall"
)

func hiddenProcessAttr() *syscall.SysProcAttr {
	return nil
}

func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}
