package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncChannelModelsFromRawJSON(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	channelID := newID()
	raw := `{
		"models": "gpt-4o-mini, deepseek-chat",
		"config": "{\"model_mapping\":{\"qwen-plus\":\"qwen/qwen-plus\"}}"
	}`
	_, err = app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, raw_json, created_at, updated_at)
		VALUES (?, '1', 'Raw Relay', 'https://raw.example', 'enabled', 'newapi', ?, ?, ?)
	`, channelID, raw, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	records, err := app.loadChannelModelSyncRecords(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}

	item := app.syncChannelModels(context.Background(), records[0])
	if item.Status != "raw_only" {
		t.Fatalf("expected raw_only, got %+v", item)
	}
	if item.ModelCount != 3 {
		t.Fatalf("expected 3 models, got %+v", item)
	}

	channel, err := app.loadChannelByID(context.Background(), channelID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.ModelCount != 3 || channel.ModelsStatus != "raw_only" {
		t.Fatalf("expected persisted raw models, got %+v", channel)
	}
}

func TestSyncChannelModelsPrefersLiveModelsWithChannelKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("authorization") != "Bearer sk-channel" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"claude-3-5-haiku"}]}`))
	}))
	defer server.Close()

	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.client = server.Client()
	app.allowLocalOutbound = true

	channelKey, err := app.encryptText("sk-channel")
	if err != nil {
		t.Fatal(err)
	}
	channelID := newID()
	_, err = app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, channel_key_encrypted, raw_json, created_at, updated_at)
		VALUES (?, '2', 'Live Relay', ?, 'enabled', 'newapi', ?, '{"models":"deepseek-chat"}', ?, ?)
	`, channelID, server.URL, channelKey, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	records, err := app.loadChannelModelSyncRecords(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	item := app.syncChannelModels(context.Background(), records[0])
	if item.Status != "live_key" {
		t.Fatalf("expected live_key, got %+v", item)
	}
	if item.ModelCount != 2 || item.SampleModels[0] != "gpt-4o-mini" {
		t.Fatalf("expected live models, got %+v", item)
	}
	if item.LatencyMs < 0 {
		t.Fatalf("expected non-negative latency, got %+v", item)
	}
}
