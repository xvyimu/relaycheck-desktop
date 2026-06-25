//go:build windows

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const startupShortcutFileName = "RelayCheck Desktop.lnk"

func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func netListen(address string) (net.Listener, error) {
	return net.Listen("tcp", address)
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

// StartupShortcutPath returns the absolute path of the .lnk shortcut that
// lives in the shell:startup folder.
func StartupShortcutPath() (string, error) {
	folder, err := startupFolder()
	if err != nil {
		return "", err
	}
	return filepath.Join(folder, startupShortcutFileName), nil
}

// CreateStartupShortcut authors a .lnk shortcut in the shell:startup folder
// pointing at the currently running executable. It drives the Windows Script
// Host Shell COM object (WScript.Shell, which implements IShellLink /
// IPersistFile) through PowerShell to author the .lnk file — the canonical
// Windows COM way to create shell shortcuts without pulling in native OLE
// bindings. The shortcut launches the app minimized so it does not steal focus
// on login.
func CreateStartupShortcut() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法获取可执行文件路径: %w", err)
	}
	shortcutPath, err := StartupShortcutPath()
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
		escapePowerShellSingleQuote(shortcutPath),
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
	if _, err := os.Stat(shortcutPath); err != nil {
		return fmt.Errorf("快捷方式创建后未找到: %w", err)
	}
	return nil
}

// RemoveStartupShortcut deletes the startup .lnk file if it exists. It is a
// no-op when the shortcut is already absent.
func RemoveStartupShortcut() error {
	shortcutPath, err := StartupShortcutPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(shortcutPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.Remove(shortcutPath); err != nil {
		return fmt.Errorf("删除快捷方式失败: %w", err)
	}
	return nil
}

// IsStartupShortcutPresent reports whether the startup .lnk file currently
// exists on disk.
func IsStartupShortcutPresent() bool {
	shortcutPath, err := StartupShortcutPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(shortcutPath)
	return err == nil
}

// escapePowerShellSingleQuote escapes a string for safe embedding inside a
// PowerShell single-quoted string literal.
func escapePowerShellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
