//go:build windows

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const startupShortcutFileName = "RelayCheck Desktop.lnk"

// hiddenProcessAttr returns a SysProcAttr that hides the child window. It is
// the autostart-domain copy of the equivalent helper in core; duplicated so
// this package stays free of any core import. The two definitions are kept
// in sync.
func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

// startupFolder returns the user's shell:startup directory, i.e.
// %APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup.
func startupFolder() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("环境变量 APPDATA 未设置，无法定位启动目录")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

// shortcutPath returns the absolute path of the .lnk shortcut that lives in
// the shell:startup folder.
func shortcutPath() (string, error) {
	folder, err := startupFolder()
	if err != nil {
		return "", err
	}
	return filepath.Join(folder, startupShortcutFileName), nil
}

// createStartupShortcut authors a .lnk shortcut in the shell:startup folder
// pointing at the currently running executable. It drives the Windows Script
// Host Shell COM object (WScript.Shell, which implements IShellLink /
// IPersistFile) through PowerShell to author the .lnk file — the canonical
// Windows COM way to create shell shortcuts without pulling in native OLE
// bindings. The shortcut launches the app minimized so it does not steal focus
// on login.
func createStartupShortcut() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}
	shortcut, err := shortcutPath()
	if err != nil {
		return err
	}
	folder, _ := startupFolder()
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("创建启动目录失败: %w", err)
	}

	// PowerShell single-quoted strings escape a literal ' as ''.
	script := fmt.Sprintf(
		`$ErrorActionPreference='Stop'; `+
			`$ws = New-Object -ComObject WScript.Shell; `+
			`$lnk = $ws.CreateShortcut('%s'); `+
			`$lnk.TargetPath = '%s'; `+
			`$lnk.WorkingDirectory = '%s'; `+
			`$lnk.Description = '%s'; `+
			`$lnk.WindowStyle = 7; `+
			`$lnk.Save()`,
		escapePowerShellSingleQuote(shortcut),
		escapePowerShellSingleQuote(exePath),
		escapePowerShellSingleQuote(filepath.Dir(exePath)),
		"RelayCheck Desktop 开机自启",
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = hiddenProcessAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建快捷方式失败: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Verify the shortcut was actually written.
	if _, err := os.Stat(shortcut); err != nil {
		return fmt.Errorf("快捷方式创建后未找到: %w", err)
	}
	return nil
}

// removeStartupShortcut deletes the startup .lnk file if it exists. It is a
// no-op when the shortcut is already absent.
func removeStartupShortcut() error {
	shortcut, err := shortcutPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(shortcut); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.Remove(shortcut); err != nil {
		return fmt.Errorf("删除快捷方式失败: %w", err)
	}
	return nil
}

// isStartupShortcutPresent reports whether the startup .lnk file currently
// exists on disk.
func isStartupShortcutPresent() bool {
	shortcut, err := shortcutPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(shortcut)
	return err == nil
}

// escapePowerShellSingleQuote escapes a string for safe embedding inside a
// PowerShell single-quoted string literal.
func escapePowerShellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
