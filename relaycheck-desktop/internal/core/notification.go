package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"relaycheck-desktop/internal/notifications"
)

// Compile-time assertion that *App satisfies the notifications package's
// NotificationHTTPPort interface (ValidateOutboundURL + DoHTTPWithTimeout).
// The adapter methods live in url_safety.go and network.go.
var _ notifications.NotificationHTTPPort = (*App)(nil)

// ==================== App 方法（转发至 NotificationHub）====================

func (a *App) reloadNotificationConfig(ctx context.Context) error {
	return a.notificationHub.Reload(ctx)
}

// ReloadNotificationConfig is the exported adapter used by the backup domain
// (backup.Infra) to re-read notification channel configuration after an
// encrypted import. It delegates to the unexported reloadNotificationConfig
// used by the rest of core.
func (a *App) ReloadNotificationConfig(ctx context.Context) error {
	return a.reloadNotificationConfig(ctx)
}

func (a *App) loadNotificationChannelsConfig(ctx context.Context) (notifications.ChannelsConfig, error) {
	return a.notificationHub.LoadConfig(ctx)
}

func (a *App) currentNotificationChannelsConfig() notifications.ChannelsConfig {
	return a.notificationHub.CurrentConfig()
}

func (a *App) buildChannelFromConfig(entry notifications.ChannelEntry) notifications.Channel {
	return a.notificationHub.BuildChannel(entry)
}

func (a *App) encryptChannelEntrySecrets(entry *notifications.ChannelEntry) error {
	return a.notificationHub.EncryptEntrySecrets(entry)
}

func (a *App) decryptChannelEntrySecrets(entry *notifications.ChannelEntry) error {
	return a.notificationHub.DecryptEntrySecrets(entry)
}

func (a *App) dispatchNotification(kind, level, title, content string) {
	a.notificationHub.Dispatch(kind, level, title, content)
}

// ==================== 通知入库与去重（从 routes.go 归拢）====================

func (a *App) notify(kind, level, title, content, relatedType, relatedID string) {
	ctx := a.rootCtx
	// Deduplicate: skip if an identical notification (same kind+relatedType+
	// relatedID+content) was inserted within the dedup window. This prevents
	// recurring events (e.g. "checkin_unsupported" for sites without a checkin
	// endpoint) from flooding the notification table on every scheduler tick.
	dedupWindow := 30 * time.Minute
	if kind == "scheduled_channel_health_probe_warning" {
		dedupWindow = 30 * time.Minute
	}
	if a.recentNotificationExists(ctx, kind, relatedType, relatedID, content, dedupWindow) {
		return
	}
	if _, execErr := a.db.ExecContext(ctx, `
		INSERT INTO app_notifications (id, type, level, title, content, read, related_type, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?)
	`, newID(), kind, level, title, content, relatedType, relatedID, now()); execErr != nil {
		log.Printf("[notify] notification insert failed: %v", execErr)
	}
	// Per-key invalidation: only dashboard-summary (unread count),
	// action-center (action items may reference notification state), and
	// checkin-status (checkin results trigger notifications) depend on
	// notification inserts. Other cached reads (accounts-list, channels-list,
	// models-overview, etc.) are unaffected.
	a.invalidateReadCacheKeys("dashboard-summary", "action-center", "checkin-status")

	// 异步分发到外部通知渠道（hub 内部用 sendWG + sendCtx 追踪，
	// Close 时会 cancel 并 Wait，无需在此处管理 goroutine 生命周期）
	a.dispatchNotification(kind, level, title, content)
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

// ==================== 通知 HTTP handler（从 routes.go 归拢）====================

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
	a.invalidateReadCacheKeys("dashboard-summary", "action-center")
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
	a.invalidateReadCacheKeys("dashboard-summary", "action-center")
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
	a.invalidateReadCacheKeys("dashboard-summary", "action-center")
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
	a.invalidateReadCacheKeys("dashboard-summary", "action-center")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ==================== 通知健康检查（从 health.go 归拢）====================

func (a *App) healthCheckNotificationChannels() HealthCheck {
	config := a.currentNotificationChannelsConfig()
	if !config.Enabled {
		return HealthCheck{ID: "notification", Label: "通知渠道", Status: "ok", Message: "外部通知未启用。"}
	}
	enabledCount := 0
	totalCount := len(config.Channels)
	for _, ch := range config.Channels {
		if ch.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return HealthCheck{ID: "notification", Label: "通知渠道", Status: "warning", Message: "外部通知已启用，但未启用任何渠道。"}
	}
	return HealthCheck{ID: "notification", Label: "通知渠道", Status: "ok", Message: fmt.Sprintf("已启用 %d/%d 个外部通知渠道。", enabledCount, totalCount)}
}

// ==================== 默认配置注入（从 app.go 归拢）====================

func withDefaultHealthNotificationTypes(valueJSON string) string {
	config, _ := notifications.ParseChannelsConfig(valueJSON)
	for index := range config.Channels {
		switch config.Channels[index].Type {
		case "webhook", "telegram":
			appendUniqueString(&config.Channels[index].Types, "scheduled_channel_health_probe_failed", 20)
			appendUniqueString(&config.Channels[index].Types, "scheduled_channel_health_probe_warning", 20)
		}
	}
	body, err := json.Marshal(config)
	if err != nil {
		return valueJSON
	}
	return string(body)
}
