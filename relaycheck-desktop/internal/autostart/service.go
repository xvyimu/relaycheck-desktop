package autostart

import (
	"os"
	"runtime"
)

// Service implements the auto-start domain. It owns the OS-level startup
// shortcut lifecycle (Windows shell:startup .lnk) and the status assembly
// logic. The host application delegates its *App handler methods to this
// Service.
//
// The auto-start domain is purely platform-based: shortcut paths are derived
// from os.Executable / environment variables, so the Service has no Infra
// dependency. Platform-specific helpers live in platform_windows.go and
// platform_other.go.
type Service struct{}

// NewService constructs an auto-start Service.
func NewService() *Service { return &Service{} }

// AutoStartStatus reports the current state of the OS-level auto-start
// configuration (Windows shell:startup .lnk shortcut).
type AutoStartStatus struct {
	// Enabled is true when the startup shortcut currently exists on disk.
	Enabled bool `json:"enabled"`
	// Supported is true on platforms where auto-start can be configured
	// (currently Windows only).
	Supported bool `json:"supported"`
	// ShortcutPath is the resolved .lnk path inside the shell:startup folder.
	ShortcutPath string `json:"shortcutPath,omitempty"`
	// TargetPath is the executable the shortcut will launch.
	TargetPath string `json:"targetPath,omitempty"`
	// Error carries the last error message, if any.
	Error string `json:"error,omitempty"`
}

// Status assembles the current auto-start state from the platform helpers.
func (s *Service) Status() AutoStartStatus {
	status := AutoStartStatus{
		Supported: runtime.GOOS == "windows",
	}
	if shortcutPath, err := shortcutPath(); err == nil {
		status.ShortcutPath = shortcutPath
	} else if status.Supported {
		status.Error = err.Error()
	}
	if exePath, err := os.Executable(); err == nil {
		status.TargetPath = exePath
	}
	status.Enabled = isStartupShortcutPresent()
	return status
}

// Enable creates the startup shortcut. On non-Windows platforms it returns
// an "unsupported" error; the host handler is responsible for mapping that
// to an appropriate HTTP status.
func (s *Service) Enable() error {
	return createStartupShortcut()
}

// Disable removes the startup shortcut. On non-Windows platforms it returns
// an "unsupported" error; the host handler is responsible for mapping that
// to an appropriate HTTP status.
func (s *Service) Disable() error {
	return removeStartupShortcut()
}
