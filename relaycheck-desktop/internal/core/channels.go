package core

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"relaycheck-desktop/internal/channels"
)

func (a *App) handleChannels(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := cachedRead(a, "channels-list", shortReadCacheTTL, func() ([]ImportedChannel, error) {
		mirror, err := a.channelsService.ListChannels(r.Context())
		if err != nil {
			return nil, err
		}
		return channelsListToCore(mirror), nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) handleBulkChannelSourceSyncStatus(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		FromStatus string `json:"fromStatus"`
		ToStatus   string `json:"toStatus"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数不完整。")
		return
	}
	fromStatus := strings.TrimSpace(input.FromStatus)
	toStatus := strings.TrimSpace(input.ToStatus)
	if !channels.IsSafeBulkSourceStatusTransition(fromStatus, toStatus) {
		writeError(w, http.StatusBadRequest, "仅支持 missing->archived、missing->active、archived->active 这几种安全批量状态切换。")
		return
	}

	affected, changedAt, err := a.channelsService.BulkSetChannelSourceSyncStatus(r.Context(), fromStatus, toStatus)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fromStatus": fromStatus,
		"toStatus":   toStatus,
		"affected":   affected,
		"changedAt":  changedAt,
	})
}

func (a *App) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	tail := pathTail(r.URL.Path, "/api/channels/")
	if r.Method == http.MethodGet {
		item, err := a.loadChannelByID(r.Context(), tail)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "渠道不存在。")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if strings.HasSuffix(tail, "/detect") {
		a.detectChannel(w, r, strings.TrimSuffix(tail, "/detect"))
		return
	}
	if strings.HasSuffix(tail, "/restore-source-status") {
		a.setChannelSourceSyncStatus(w, r, strings.TrimSuffix(tail, "/restore-source-status"), "active")
		return
	}
	if strings.HasSuffix(tail, "/archive-source-status") {
		a.setChannelSourceSyncStatus(w, r, strings.TrimSuffix(tail, "/archive-source-status"), "archived")
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

// loadChannelByID is the *App forwarder for
// channels.Service.LoadChannelByID. Converts the channels mirror type back to
// core.ImportedChannel so existing callers (handlers, scheduler, model-sync,
// health-probe tests) are unchanged.
func (a *App) loadChannelByID(ctx context.Context, id string) (ImportedChannel, error) {
	item, err := a.channelsService.LoadChannelByID(ctx, id)
	if err != nil {
		return ImportedChannel{}, err
	}
	return channelFromMirror(item), nil
}

func (a *App) setChannelSourceSyncStatus(w http.ResponseWriter, r *http.Request, id string, nextStatus string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	if nextStatus != "active" && nextStatus != "archived" {
		writeError(w, http.StatusBadRequest, "不支持的渠道同步状态。")
		return
	}

	result, err := a.channelsService.SetChannelSourceSyncStatus(r.Context(), id, nextStatus)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "渠道不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":             result.ID,
		"name":           result.Name,
		"previousStatus": result.PreviousStatus,
		"sourceStatus":   result.SourceStatus,
		"changedAt":      result.ChangedAt,
	})
}

func (a *App) detectChannel(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	result, err := a.channelsService.DetectChannel(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "渠道不存在。")
		return
	}
	if errors.Is(err, channels.ErrEmptyChannelBaseURL) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"detection": detectionFromMirror(result.Detection),
		"siteId":    result.SiteID,
		"created":   result.Created,
	})
}

// ensureUpstreamSiteForChannel is now a *App forwarder for
// sites.Service.EnsureUpstreamSiteForChannel — see scanner.go.
