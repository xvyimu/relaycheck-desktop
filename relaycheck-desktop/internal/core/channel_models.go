package core

import (
	"context"
	"fmt"
	"net/http"

	"relaycheck-desktop/internal/channels"
)

type channelModelSyncItem struct {
	ChannelID    string   `json:"channelId"`
	ChannelName  string   `json:"channelName"`
	BaseURL      string   `json:"baseUrl,omitempty"`
	Kind         string   `json:"kind"`
	HasKey       bool     `json:"hasKey"`
	Status       string   `json:"status"`
	Source       string   `json:"source,omitempty"`
	ModelCount   int      `json:"modelCount"`
	SampleModels []string `json:"sampleModels,omitempty"`
	LatencyMs    int64    `json:"latencyMs,omitempty"`
	Message      string   `json:"message,omitempty"`
	LastSyncedAt string   `json:"lastSyncedAt,omitempty"`
}

// channelModelSyncRecord is the persisted state for one channel during a
// model sync batch. Kept in core because task_runner.loadChannelModelSyncRecordsForHealthSite
// produces it and feeds it to (*App).syncChannelModels.
type channelModelSyncRecord struct {
	ID                  string
	Name                string
	BaseURL             string
	Kind                string
	RawJSON             string
	ChannelKeyEncrypted string
	ModelCount          int
	SampleModelsJSON    string
	ModelsSource        string
	ModelsStatus        string
	ModelsLastSyncedAt  string
	ModelsMessage       string
}

func (a *App) handleChannelModelsOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	items, err := a.loadChannelModelItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	overview := channels.BuildChannelModelOverview(channelModelSyncItemsToMirror(items), 0, now())
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleChannelModelsSync(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		Limit int `json:"limit"`
	}
	if r.ContentLength != 0 {
		_ = decodeJSON(r, &input)
	}
	input.Limit = clampBatchLimit(input.Limit, 10)
	records, err := a.loadChannelModelSyncRecords(r.Context(), input.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]channelModelSyncItem, 0, len(records))
	for _, record := range records {
		items = append(items, a.syncChannelModels(r.Context(), record))
	}
	overview := channels.BuildChannelModelOverview(channelModelSyncItemsToMirror(items), len(records), now())
	if len(records) > 0 {
		a.notify("channel_models_synced", "success", "渠道模型同步完成", fmt.Sprintf("已同步 %d 个渠道的模型覆盖。", len(records)), "channel", "")
	}
	writeJSON(w, http.StatusOK, overview)
}

// loadChannelModelSyncRecords is the *App forwarder for
// channels.Service.LoadChannelModelSyncRecords. Converts the channels mirror
// type back to core channelModelSyncRecord so existing callers (tests,
// task_runner.syncModelsForHealthSite) are unchanged.
func (a *App) loadChannelModelSyncRecords(ctx context.Context, limit int) ([]channelModelSyncRecord, error) {
	mirror, err := a.channelsService.LoadChannelModelSyncRecords(ctx, limit)
	if err != nil {
		return nil, err
	}
	if len(mirror) == 0 {
		return []channelModelSyncRecord{}, nil
	}
	out := make([]channelModelSyncRecord, 0, len(mirror))
	for _, m := range mirror {
		out = append(out, channelModelSyncRecord{
			ID:                  m.ID,
			Name:                m.Name,
			BaseURL:             m.BaseURL,
			Kind:                m.Kind,
			RawJSON:             m.RawJSON,
			ChannelKeyEncrypted: m.ChannelKeyEncrypted,
			ModelCount:          m.ModelCount,
			SampleModelsJSON:    m.SampleModelsJSON,
			ModelsSource:        m.ModelsSource,
			ModelsStatus:        m.ModelsStatus,
			ModelsLastSyncedAt:  m.ModelsLastSyncedAt,
			ModelsMessage:       m.ModelsMessage,
		})
	}
	return out, nil
}

// loadChannelModelItems is the *App forwarder for
// channels.Service.LoadChannelModelItems. Converts the channels mirror type
// back to core channelModelSyncItem so existing callers are unchanged.
func (a *App) loadChannelModelItems(ctx context.Context) ([]channelModelSyncItem, error) {
	mirror, err := a.channelsService.LoadChannelModelItems(ctx)
	if err != nil {
		return nil, err
	}
	return channelModelSyncItemsToCore(mirror), nil
}

// syncChannelModels is the *App forwarder for
// channels.Service.SyncChannelModels. Converts the core channelModelSyncRecord
// to the channels mirror type and the channels.ChannelModelSyncItem result
// back to core channelModelSyncItem so existing callers (tests, task_runner)
// are unchanged.
func (a *App) syncChannelModels(ctx context.Context, record channelModelSyncRecord) channelModelSyncItem {
	mirror := a.channelsService.SyncChannelModels(ctx, channelModelSyncRecordToMirror(record))
	return channelModelSyncItemFromMirror(mirror)
}
