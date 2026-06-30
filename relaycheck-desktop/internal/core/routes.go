package core

import (
	"context"
	"net/http"
	"strings"
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

func pathTail(path, prefix string) string {
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}
