package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	productName    = "RelayCheck Desktop"
	productVersion = "v1.0"
	buildTime      = "local build"
)

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", a.handleLogin)
	mux.HandleFunc("/api/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/auth/session", a.handleSession)
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/system/status", a.requireSession(a.handleSystemStatus))
	mux.HandleFunc("/api/system/settings", a.requireSession(a.handleSystemSettings))
	mux.HandleFunc("/api/system/scheduler-status", a.requireSession(a.handleSystemSchedulerStatus))
	mux.HandleFunc("/api/system/proxy-test", a.requireSession(a.handleSystemProxyTest))
	mux.HandleFunc("/api/system/diagnostics", a.requireSession(a.handleSystemDiagnostics))
	mux.HandleFunc("/api/system/action-center", a.requireSession(a.handleActionCenter))
	mux.HandleFunc("/api/system/audit-log", a.requireSession(a.handleAuditLog))
	mux.HandleFunc("/api/system/backups", a.requireSession(a.handleSystemBackups))
	mux.HandleFunc("/api/system/backup", a.requireSession(a.handleSystemBackup))
	mux.HandleFunc("/api/system/backups/delete", a.requireSession(a.handleSystemDeleteBackups))
	mux.HandleFunc("/api/system/restore", a.requireSession(a.handleSystemRestore))
	mux.HandleFunc("/api/system/migrate-from-python-db", a.requireSession(a.handleMigrateFromPythonDB))
	mux.HandleFunc("/api/local-newapi", a.requireSession(a.handleLocalNewAPIInstances))
	mux.HandleFunc("/api/local-newapi/scan", a.requireSession(a.handleScanLocalNewAPI))
	mux.HandleFunc("/api/local-newapi/import-from-sqlite", a.requireSession(a.handleImportFromSQLite))
	mux.HandleFunc("/api/local-newapi/import-from-admin-api", a.requireSession(a.handleImportFromAdminAPI))
	mux.HandleFunc("/api/local-newapi/", a.requireSession(a.handleLocalNewAPIInstanceByID))
	mux.HandleFunc("/api/channels", a.requireSession(a.handleChannels))
	mux.HandleFunc("/api/channels/bulk-source-status", a.requireSession(a.handleBulkChannelSourceSyncStatus))
	mux.HandleFunc("/api/channels/models/overview", a.requireSession(a.handleChannelModelsOverview))
	mux.HandleFunc("/api/channels/models/sync", a.requireSession(a.handleChannelModelsSync))
	mux.HandleFunc("/api/channels/", a.requireSession(a.handleChannelByID))
	mux.HandleFunc("/api/upstream-sites", a.requireSession(a.handleUpstreamSites))
	mux.HandleFunc("/api/upstream-sites/bulk-detect", a.requireSession(a.handleBulkDetectUpstreamSites))
	mux.HandleFunc("/api/upstream-sites/", a.requireSession(a.handleUpstreamSiteByID))
	mux.HandleFunc("/api/accounts", a.requireSession(a.handleAccounts))
	mux.HandleFunc("/api/accounts/bulk-open-browser-login", a.requireSession(a.handleBulkOpenBrowserLogin))
	mux.HandleFunc("/api/accounts/bulk-finish-browser-login", a.requireSession(a.handleBulkFinishBrowserLogin))
	mux.HandleFunc("/api/accounts/bulk-password-login", a.requireSession(a.handleBulkPasswordLogin))
	mux.HandleFunc("/api/accounts/bulk-test-api-keys", a.requireSession(a.handleBulkTestAPIKeys))
	mux.HandleFunc("/api/accounts/bulk-refresh-balances", a.requireSession(a.handleBulkRefreshBalances))
	mux.HandleFunc("/api/accounts/delete-unsupported-checkins", a.requireSession(a.handleDeleteUnsupportedCheckinAccounts))
	mux.HandleFunc("/api/accounts/import-legacy-config", a.requireSession(a.handleLegacyConfigImport))
	mux.HandleFunc("/api/accounts/import-chrome-passwords/preview", a.requireSession(a.handleChromePasswordImportPreview))
	mux.HandleFunc("/api/accounts/import-chrome-passwords/import", a.requireSession(a.handleChromePasswordImport))
	mux.HandleFunc("/api/accounts/", a.requireSession(a.handleAccountByID))
	mux.HandleFunc("/api/models/overview", a.requireSession(a.handleModelOverview))
	mux.HandleFunc("/api/models/sync", a.requireSession(a.handleModelSync))
	mux.HandleFunc("/api/models/pricing", a.requireSession(a.handleModelPricing))
	mux.HandleFunc("/api/models/pricing/sync", a.requireSession(a.handleModelPricingSync))
	mux.HandleFunc("/api/keys/export-preview", a.requireSession(a.handleKeyExportPreview))
	mux.HandleFunc("/api/checkins/today", a.requireSession(a.handleTodayCheckins))
	mux.HandleFunc("/api/checkins/logs", a.requireSession(a.handleCheckinLogs))
	mux.HandleFunc("/api/checkins/status", a.requireSession(a.handleCheckinStatus))
	mux.HandleFunc("/api/checkins/run-all", a.requireSession(a.handleRunAllCheckins))
	mux.HandleFunc("/api/usage/overview", a.requireSession(a.handleUsageOverview))
	mux.HandleFunc("/api/balances/snapshots", a.requireSession(a.handleBalanceSnapshots))
	mux.HandleFunc("/api/notifications", a.requireSession(a.handleNotifications))
	mux.HandleFunc("/api/notifications/mark-all-read", a.requireSession(a.handleMarkAllNotificationsRead))
	mux.HandleFunc("/api/notifications/clear-read", a.requireSession(a.handleClearReadNotifications))
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	userID, err := a.withSession(r)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"userId":        userID,
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Username == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "登录参数不完整。")
		return
	}

	var userID, hash, displayName string
	err := a.db.QueryRowContext(r.Context(), `
		SELECT id, password_hash, COALESCE(display_name, username)
		FROM app_users WHERE username = ?
	`, input.Username).Scan(&userID, &hash, &displayName)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		a.audit("auth.login_failed", "warning", input.Username, "user", "", "登录失败："+input.Username, nil)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误。")
		return
	}

	token := randomToken()
	a.mu.Lock()
	a.sessions[token] = userID
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "relaycheck_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
	a.audit("auth.login", "info", input.Username, "user", userID, "登录成功："+displayName, nil)
	writeJSON(w, http.StatusOK, map[string]string{
		"userId":      userID,
		"username":    input.Username,
		"displayName": displayName,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	if cookie, err := r.Cookie("relaycheck_session"); err == nil {
		a.mu.Lock()
		userID := a.sessions[cookie.Value]
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
		a.audit("auth.logout", "info", userID, "user", userID, "退出登录", nil)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "relaycheck_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	status, err := a.systemStatus(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *App) systemStatus(r *http.Request) (SystemStatus, error) {
	summary, err := a.dashboardSummary(r)
	if err != nil {
		return SystemStatus{}, err
	}
	diagnostics, err := a.systemDiagnostics(r)
	if err != nil {
		return SystemStatus{}, err
	}
	a.mu.RLock()
	bind := a.bind
	port := a.port
	a.mu.RUnlock()
	return SystemStatus{
		ProductName:    productName,
		ProductVersion: productVersion,
		BuildTime:      buildTime,
		Architecture:   "Go + embedded React + SQLite",
		BindAddress:    bind,
		Port:           port,
		DatabasePath:   a.databasePath(),
		BackupDir:      a.backupsDir(),
		NetworkProxy:   a.networkProxyStatus(),
		Scheduler:      a.buildSchedulerStatus(r.Context()),
		LastDiagnostics: SystemStatusDiagnostics{
			Overall:     diagnostics.Overall,
			GeneratedAt: diagnostics.GeneratedAt,
			ItemCount:   len(diagnostics.Items),
		},
		Summary: summary,
	}, nil
}

func (a *App) handleSystemSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, a.buildSchedulerStatus(r.Context()))
}

func (a *App) dashboardSummary(r *http.Request) (DashboardSummary, error) {
	return cachedRead(a, "dashboard-summary", shortReadCacheTTL, func() (DashboardSummary, error) {
		return a.buildDashboardSummary(r.Context())
	})
}

func (a *App) buildDashboardSummary(ctx context.Context) (DashboardSummary, error) {
	var summary DashboardSummary
	queries := []struct {
		target *int
		sql    string
	}{
		{&summary.LocalNewAPICount, `SELECT COUNT(*) FROM local_newapi_instances`},
		{&summary.ImportedChannelCount, `SELECT COUNT(*) FROM imported_channels`},
		{&summary.IdentifiedChannelCount, `SELECT COUNT(*) FROM imported_channels WHERE upstream_kind <> 'unknown'`},
		{&summary.AccountCount, `SELECT COUNT(*) FROM channel_accounts`},
		{&summary.UnreadNotifications, `SELECT COUNT(*) FROM app_notifications WHERE read = 0`},
	}
	for _, query := range queries {
		if err := a.db.QueryRowContext(ctx, query.sql).Scan(query.target); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, type, level, title, content, read, created_at
		FROM app_notifications
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []Notification{}
	for rows.Next() {
		var item Notification
		var read int
		if err := rows.Scan(&item.ID, &item.Type, &item.Level, &item.Title, &item.Content, &read, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		item.Read = read == 1
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE read = 0`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleClearReadNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	_, err := a.db.ExecContext(r.Context(), `DELETE FROM app_notifications WHERE read = 1`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) notify(kind, level, title, content, relatedType, relatedID string) {
	_, _ = a.db.Exec(`
		INSERT INTO app_notifications (id, type, level, title, content, read, related_type, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, newID(), kind, level, title, content, relatedType, relatedID, now())
	a.invalidateReadCache()

	// 异步分发到外部通知渠道
	go a.dispatchNotification(kind, level, title, content)
}

func randomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func pathTail(path, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}
