package core

import (
	"net/http"
	"runtime"
)

// handleSystemAutoStart exposes GET /api/system/autostart and
// PUT /api/system/autostart. GET returns the current auto-start status;
// PUT { "enabled": true|false } creates or removes the shell:startup shortcut.
func (a *App) handleSystemAutoStart(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.autostartService.Status())
	case http.MethodPut:
		a.handleUpdateAutoStart(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleUpdateAutoStart forwards the PUT /api/system/autostart request to the
// autostart service. The platform check (501 on non-Windows) and audit
// logging stay in this forwarding layer because they are HTTP/host concerns;
// the shortcut lifecycle lives in the autostart package.
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
		if err := a.autostartService.Enable(); err != nil {
			a.audit("autostart.enable_failed", "warning", "", "system", "autostart", "开启开机自启失败："+err.Error(), map[string]interface{}{"enabled": true, "error": err.Error()})
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.audit("autostart.enabled", "info", "", "system", "autostart", "已开启开机自启。", map[string]interface{}{"enabled": true})
	} else {
		if err := a.autostartService.Disable(); err != nil {
			a.audit("autostart.disable_failed", "warning", "", "system", "autostart", "关闭开机自启失败："+err.Error(), map[string]interface{}{"enabled": false, "error": err.Error()})
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.audit("autostart.disabled", "info", "", "system", "autostart", "已关闭开机自启。", map[string]interface{}{"enabled": false})
	}

	writeJSON(w, http.StatusOK, a.autostartService.Status())
}
