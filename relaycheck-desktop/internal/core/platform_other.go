//go:build !windows

package core

import (
	"errors"
	"net"
	"syscall"
)

func hiddenProcessAttr() *syscall.SysProcAttr {
	return nil
}

func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
}

// StartupShortcutPath is unsupported on non-Windows platforms.
func StartupShortcutPath() (string, error) {
	return "", errors.New("开机自启仅支持 Windows 平台")
}

// CreateStartupShortcut is unsupported on non-Windows platforms.
func CreateStartupShortcut() error {
	return errors.New("开机自启仅支持 Windows 平台")
}

// RemoveStartupShortcut is unsupported on non-Windows platforms.
func RemoveStartupShortcut() error {
	return errors.New("开机自启仅支持 Windows 平台")
}

// IsStartupShortcutPresent always returns false on non-Windows platforms.
func IsStartupShortcutPresent() bool {
	return false
}
