package core

import (
	"encoding/json"
	"net/http"
)

func (a *App) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, action, level, COALESCE(actor,''), COALESCE(resource_type,''), COALESCE(resource_id,''),
		       summary, COALESCE(metadata_json,''), created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	items := []AuditLogItem{}
	for rows.Next() {
		var item AuditLogItem
		if err := rows.Scan(&item.ID, &item.Action, &item.Level, &item.Actor, &item.ResourceType, &item.ResourceID, &item.Summary, &item.MetadataJSON, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) audit(action string, level string, actor string, resourceType string, resourceID string, summary string, metadata map[string]interface{}) {
	if a == nil || a.db == nil {
		return
	}
	if level == "" {
		level = "info"
	}
	metadataJSON := ""
	if len(metadata) > 0 {
		if data, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(data)
		}
	}
	_, _ = a.db.Exec(`
		INSERT INTO audit_log (id, action, level, actor, resource_type, resource_id, summary, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), action, level, actor, resourceType, resourceID, summary, metadataJSON, now())
}
