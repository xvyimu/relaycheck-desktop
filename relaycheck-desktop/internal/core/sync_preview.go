package core

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"relaycheck-desktop/internal/accounts"
)

// localNewAPISyncSourceInput is declared in local_newapi.go (alongside
// localNewAPISyncRunInput) because the scheduler constructs both directly.
// This file only provides the normalizeLocalNewAPISyncSourceInput helper and
// the *App forwarders that the sync-preview handlers delegate to.

// normalizeLocalNewAPISyncSourceInput applies defaults to a core
// localNewAPISyncSourceInput. Kept in core because the sync-preview handlers
// decode the JSON body into the core type before forwarding to the accounts
// service.
func normalizeLocalNewAPISyncSourceInput(input *localNewAPISyncSourceInput) {
	if strings.TrimSpace(input.UserID) == "" {
		input.UserID = "1"
	}
	input.PageSize = clampInt(input.PageSize, 10, 100, 100)
}

func (a *App) previewLocalNewAPIInstanceSync(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncSourceInput
	_ = decodeJSON(r, &input)
	normalizeLocalNewAPISyncSourceInput(&input)

	instance, err := a.getLocalNewAPIInstance(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "NewAPI 实例不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	source, records, err := a.sourceChannelRecordsForLocalNewAPI(r.Context(), instance, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	preview, err := a.buildLocalNewAPISyncPreview(r.Context(), instance, source, records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *App) markMissingLocalNewAPIInstance(w http.ResponseWriter, r *http.Request, id string) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input localNewAPISyncSourceInput
	_ = decodeJSON(r, &input)
	normalizeLocalNewAPISyncSourceInput(&input)

	instance, err := a.getLocalNewAPIInstance(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "NewAPI 实例不存在。")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := a.reconcileMissingLocalNewAPIInstance(r.Context(), instance, input, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// reconcileMissingLocalNewAPIInstance is the *App forwarder for
// accounts.Service.ReconcileMissingLocalNewAPIInstance. Converts the core
// input/instance types to their accounts mirrors. Used by the mark-missing
// handler and the scheduler (runScheduledLocalNewAPISync).
func (a *App) reconcileMissingLocalNewAPIInstance(ctx context.Context, instance LocalNewAPIInstance, input localNewAPISyncSourceInput, notify bool) (map[string]interface{}, error) {
	return a.accountsService.ReconcileMissingLocalNewAPIInstance(ctx, localNewAPIInstanceToMirror(instance), accounts.SyncSourceInput{
		AccessToken:      input.AccessToken,
		SaveAccessToken:  input.SaveAccessToken,
		ClearAccessToken: input.ClearAccessToken,
		UserID:           input.UserID,
		PageSize:         input.PageSize,
	}, notify)
}

// sourceChannelRecordsForLocalNewAPI is the *App forwarder for
// accounts.Service.SourceChannelRecordsForLocalNewAPI. Converts the core
// instance type to its accounts mirror. Used by the sync-preview handler.
func (a *App) sourceChannelRecordsForLocalNewAPI(ctx context.Context, instance LocalNewAPIInstance, input localNewAPISyncSourceInput) (string, []map[string]interface{}, error) {
	return a.accountsService.SourceChannelRecordsForLocalNewAPI(ctx, localNewAPIInstanceToMirror(instance), accounts.SyncSourceInput{
		AccessToken:      input.AccessToken,
		SaveAccessToken:  input.SaveAccessToken,
		ClearAccessToken: input.ClearAccessToken,
		UserID:           input.UserID,
		PageSize:         input.PageSize,
	})
}

// buildLocalNewAPISyncPreview is the *App forwarder for
// accounts.Service.BuildLocalNewAPISyncPreview. Converts the core instance
// type to its accounts mirror, and the returned accounts.SyncPreview back to
// the core LocalNewAPISyncPreview type. Used by the sync-preview handler and
// the sync-preview tests.
func (a *App) buildLocalNewAPISyncPreview(ctx context.Context, instance LocalNewAPIInstance, source string, records []map[string]interface{}) (LocalNewAPISyncPreview, error) {
	mirror, err := a.accountsService.BuildLocalNewAPISyncPreview(ctx, localNewAPIInstanceToMirror(instance), source, records)
	if err != nil {
		return LocalNewAPISyncPreview{}, err
	}
	return localNewAPISyncPreviewFromMirror(mirror), nil
}

// localNewAPIInstanceToMirror converts a core.LocalNewAPIInstance to its
// accounts.LocalNewAPIInstance mirror so the accounts service stays decoupled
// from core types. The inverse of localNewAPIInstanceFromMirror in
// local_newapi.go.
func localNewAPIInstanceToMirror(instance LocalNewAPIInstance) accounts.LocalNewAPIInstance {
	return accounts.LocalNewAPIInstance{
		ID:                 instance.ID,
		Name:               instance.Name,
		BaseURL:            instance.BaseURL,
		DetectedFrom:       instance.DetectedFrom,
		Status:             instance.Status,
		Version:            instance.Version,
		DatabasePath:       instance.DatabasePath,
		ChannelCount:       instance.ChannelCount,
		HasSyncToken:       instance.HasSyncToken,
		SyncTokenMasked:    instance.SyncTokenMasked,
		LastScannedAt:      instance.LastScannedAt,
		CreatedAt:          instance.CreatedAt,
		UpdatedAt:          instance.UpdatedAt,
		SyncCapability:     instance.SyncCapability,
		SyncTokenEncrypted: instance.SyncTokenEncrypted,
	}
}

// localNewAPISyncPreviewFromMirror converts an accounts.SyncPreview back to
// the core LocalNewAPISyncPreview type so handlers and tests keep using the
// core type.
func localNewAPISyncPreviewFromMirror(m accounts.SyncPreview) LocalNewAPISyncPreview {
	items := make([]SyncPreviewItem, 0, len(m.Items))
	for _, it := range m.Items {
		items = append(items, SyncPreviewItem{
			SourceChannelID: it.SourceChannelID,
			Name:            it.Name,
			BaseURL:         it.BaseURL,
			Status:          it.Status,
			UpstreamKind:    it.UpstreamKind,
			Action:          it.Action,
			Reason:          it.Reason,
			ChangedFields:   it.ChangedFields,
		})
	}
	return LocalNewAPISyncPreview{
		InstanceID:     m.InstanceID,
		InstanceName:   m.InstanceName,
		Source:         m.Source,
		Total:          m.Total,
		NewCount:       m.NewCount,
		ChangedCount:   m.ChangedCount,
		UnchangedCount: m.UnchangedCount,
		SkippedCount:   m.SkippedCount,
		RemovedCount:   m.RemovedCount,
		Items:          items,
		GeneratedAt:    m.GeneratedAt,
	}
}
