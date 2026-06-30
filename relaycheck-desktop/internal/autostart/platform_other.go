//go:build !windows

package autostart

import "errors"

// shortcutPath is unsupported on non-Windows platforms.
func shortcutPath() (string, error) {
	return "", errors.New("开机自启仅支持 Windows 平台")
}

// createStartupShortcut is unsupported on non-Windows platforms.
func createStartupShortcut() error {
	return errors.New("开机自启仅支持 Windows 平台")
}

// removeStartupShortcut is unsupported on non-Windows platforms.
func removeStartupShortcut() error {
	return errors.New("开机自启仅支持 Windows 平台")
}

// isStartupShortcutPresent always returns false on non-Windows platforms.
func isStartupShortcutPresent() bool {
	return false
}
