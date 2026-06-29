package core

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	productName    = "RelayCheck Desktop"
	productVersion = "v1.1.0"
	buildTime      = "local build"
)

// RegisterRoutes registers all HTTP handlers on the given mux.
func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", a.handleHealth)
	mux.HandleFunc("/api/analytics", a.requireSession(a.handleAnalytics))
	mux.HandleFunc("/api/scheduler/channel-schedules", a.requireSession(a.handleChannelSchedules))
	mux.HandleFunc("/api/scheduler/calendar", a.requireSession(a.handleScheduleCalendar))
	mux.HandleFunc("/api/scheduler/next-runs", a.requireSession(a.handleNextRuns))
	mux.HandleFunc("/api/tasks/dry-run", a.requireSession(a.handleDryRun))
	mux.HandleFunc("/api/tasks/start", a.requireSession(a.handleTaskStart))
	mux.HandleFunc("/api/tasks/", a.requireSession(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/stream") {
			a.handleTaskStream(w, r)
		} else if strings.HasSuffix(path, "/cancel") {
			a.handleTaskCancel(w, r)
		} else {
			writeError(w, http.StatusNotFound, "未知的任务端点。")
		}
	}))
	mux.HandleFunc("/api/system/status", a.requireSession(a.handleSystemStatus))
	mux.HandleFunc("/api/system/version-check", a.requireSession(a.handleVersionCheck))
	mux.HandleFunc("/api/system/autostart", a.requireSession(a.handleSystemAutoStart))
	mux.HandleFunc("/api/system/legacy-check", a.requireSession(a.handleLegacyPythonCheck))
	mux.HandleFunc("/api/system/port-check", a.requireSession(a.handleSystemPortCheck))
	mux.HandleFunc("/api/system/settings", a.requireSession(a.handleSystemSettings))
	mux.HandleFunc("/api/system/scheduler-status", a.requireSession(a.handleSystemSchedulerStatus))
	mux.HandleFunc("/api/system/proxy-test", a.requireSession(a.handleSystemProxyTest))
	mux.HandleFunc("/api/system/diagnostics", a.requireSession(a.handleSystemDiagnostics))
	mux.HandleFunc("/api/system/action-center", a.requireSession(a.handleActionCenter))
	mux.HandleFunc("/api/system/audit-log", a.requireSession(a.handleAuditLog))
	mux.HandleFunc("/api/system/backups", a.requireSession(a.handleSystemBackups))
	mux.HandleFunc("/api/system/backup", a.requireSession(a.handleSystemBackup))
	mux.HandleFunc("/api/system/export", a.requireSession(a.handleEncryptedExport))
	mux.HandleFunc("/api/system/import", a.requireSession(a.handleEncryptedImport))
	mux.HandleFunc("/api/system/exports", a.requireSession(a.handleListExports))
	mux.HandleFunc("/api/system/backups/delete", a.requireSession(a.handleSystemDeleteBackups))
	mux.HandleFunc("/api/system/restore", a.requireSession(a.handleSystemRestore))
	mux.HandleFunc("/api/local-newapi", a.requireSession(a.handleLocalNewAPIInstances))
	mux.HandleFunc("/api/local-newapi/scan", a.requireSession(a.handleScanLocalNewAPI))
	mux.HandleFunc("/api/local-newapi/import-from-sqlite", a.requireSession(a.handleImportFromSQLite))
	mux.HandleFunc("/api/local-newapi/import-from-admin-api", a.requireSession(a.handleImportFromAdminAPI))
	mux.HandleFunc("/api/local-newapi/auto-detect-import", a.requireSession(a.handleAutoDetectAndImport))
	mux.HandleFunc("/api/local-newapi/", a.requireSession(a.handleLocalNewAPIInstanceByID))
	mux.HandleFunc("/api/channels", a.requireSession(a.handleChannels))
	mux.HandleFunc("/api/channels/bulk-source-status", a.requireSession(a.handleBulkChannelSourceSyncStatus))
	mux.HandleFunc("/api/channels/health/overview", a.requireSession(a.handleChannelHealthOverview))
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
	mux.HandleFunc("/api/notifications/mark-read", a.requireSession(a.handleMarkNotificationRead))
	mux.HandleFunc("/api/notifications/trim", a.requireSession(a.handleTrimNotifications))
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
	preferredPort := a.preferredPort
	portConflict := a.portConflict
	a.mu.RUnlock()
	return SystemStatus{
		ProductName:    productName,
		ProductVersion: productVersion,
		BuildTime:      buildTime,
		Architecture:   "Go + embedded React + SQLite",
		BindAddress:    bind,
		Port:           port,
		PreferredPort:  preferredPort,
		PortConflict:   portConflict,
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
	levelFilter := r.URL.Query().Get("level")
	typeFilter := r.URL.Query().Get("type")
	unreadOnly := r.URL.Query().Get("unread") == "1"
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	limit = clampBatchLimit(limit, 100)
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}

	query := `SELECT id, type, level, title, content, read, created_at FROM app_notifications WHERE 1=1`
	var args []interface{}
	if levelFilter != "" {
		query += ` AND level = ?`
		args = append(args, levelFilter)
	}
	if typeFilter != "" {
		query += ` AND type = ?`
		args = append(args, typeFilter)
	}
	if unreadOnly {
		query += ` AND read = 0`
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := a.db.QueryContext(r.Context(), query, args...)
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
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
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

func (a *App) handleTrimNotifications(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	keep := 10
	if k := r.URL.Query().Get("keep"); k != "" {
		if n, err := strconv.Atoi(k); err == nil && n > 0 {
			keep = n
		}
	}
	_, err := a.db.ExecContext(r.Context(),
		`DELETE FROM app_notifications WHERE id NOT IN (SELECT id FROM app_notifications ORDER BY created_at DESC LIMIT ?)`, keep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var body struct {
		ID        string `json:"id"`
		AllOfType string `json:"allOfType"` // mark all of a type as read
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	if body.AllOfType != "" {
		_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE type = ? AND read = 0`, body.AllOfType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if body.ID != "" {
		_, err := a.db.ExecContext(r.Context(), `UPDATE app_notifications SET read = 1 WHERE id = ?`, body.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	a.invalidateReadCache()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) notify(kind, level, title, content, relatedType, relatedID string) {
	// Deduplicate: skip if an identical notification (same kind+relatedType+
	// relatedID+content) was inserted within the dedup window. This prevents
	// recurring events (e.g. "checkin_unsupported" for sites without a checkin
	// endpoint) from flooding the notification table on every scheduler tick.
	dedupWindow := 30 * time.Minute
	if kind == "scheduled_channel_health_probe_warning" {
		dedupWindow = 30 * time.Minute
	}
	if a.recentNotificationExists(context.Background(), kind, relatedType, relatedID, content, dedupWindow) {
		return
	}
	if _, execErr := a.db.Exec(`
		INSERT INTO app_notifications (id, type, level, title, content, read, related_type, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, newID(), kind, level, title, content, relatedType, relatedID, now()); execErr != nil {
		log.Printf("[notify] notification insert failed: %v", execErr)
	}
	a.invalidateReadCache()

	// 异步分发到外部通知渠道
	go a.dispatchNotification(kind, level, title, content)
}

func (a *App) recentNotificationExists(ctx context.Context, kind string, relatedType string, relatedID string, content string, window time.Duration) bool {
	if window <= 0 {
		return false
	}
	cutoff := time.Now().Add(-window).UTC().Format(time.RFC3339Nano)
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM app_notifications
		WHERE type=?
		  AND related_type=?
		  AND related_id=?
		  AND content=?
		  AND created_at >= ?
	`, kind, relatedType, relatedID, content, cutoff).Scan(&count)
	return err == nil && count > 0
}

func pathTail(path, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}
