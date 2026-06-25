package core

import (
	"net/http"
	"os"
	"runtime"
)

// handleSystemAutoStart exposes GET /api/system/autostart and
// PUT /api/system/autostart. GET returns the current auto-start status;
// PUT { "enabled": true|false } creates or removes the shell:startup shortcut.
func (a *App) handleSystemAutoStart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, buildAutoStartStatus())
	case http.MethodPut:
		a.handleUpdateAutoStart(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) handleUpdateAutoStart(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败，请发送 {\"enabled\": true|false}。")
		return
	}

	if runtime.GOOS != "windows" {
		writeError(w, http.StatusNotImplemented, "开机自启仅支持 Windows 平台。")
		return
	}

	if input.Enabled {
		if err := CreateStartupShortcut(); err != nil {
			a.audit("autostart.enable_failed", "warning", "", "system", "autostart", "开启开机自启失败："+err.Error(), map[string]interface{}{"enabled": true, "error": err.Error()})
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.audit("autostart.enabled", "info", "", "system", "autostart", "已开启开机自启。", map[string]interface{}{"enabled": true})
	} else {
		if err := RemoveStartupShortcut(); err != nil {
			a.audit("autostart.disable_failed", "warning", "", "system", "autostart", "关闭开机自启失败："+err.Error(), map[string]interface{}{"enabled": false, "error": err.Error()})
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.audit("autostart.disabled", "info", "", "system", "autostart", "已关闭开机自启。", map[string]interface{}{"enabled": false})
	}

	writeJSON(w, http.StatusOK, buildAutoStartStatus())
}

// buildAutoStartStatus assembles the current auto-start state from the
// platform helpers.
func buildAutoStartStatus() AutoStartStatus {
	status := AutoStartStatus{
		Supported: runtime.GOOS == "windows",
	}
	if shortcutPath, err := StartupShortcutPath(); err == nil {
		status.ShortcutPath = shortcutPath
	} else if status.Supported {
		status.Error = err.Error()
	}
	if exePath, err := os.Executable(); err == nil {
		status.TargetPath = exePath
	}
	status.Enabled = IsStartupShortcutPresent()
	return status
}
