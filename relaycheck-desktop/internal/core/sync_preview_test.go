package core

import (
	"context"
	"testing"
)

// insertImportedChannelForSyncPreviewTest 插入一条本地已导入渠道，用于同步预览测试。
func insertImportedChannelForSyncPreviewTest(t *testing.T, app *App, instanceID, sourceID, name, baseURL, status, kind string) {
	t.Helper()
	_, err := app.db.Exec(`
		INSERT INTO imported_channels (id, source_channel_id, local_instance_id, name, base_url, status, upstream_kind, raw_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?)
	`, newID(), sourceID, instanceID, name, baseURL, status, kind, now(), now())
	if err != nil {
		t.Fatalf("insert imported channel %s: %v", sourceID, err)
	}
}

func findPreviewItemBySourceID(items []SyncPreviewItem, sourceID string) *SyncPreviewItem {
	for i := range items {
		if items[i].SourceChannelID == sourceID {
			return &items[i]
		}
	}
	return nil
}

// TestBuildLocalNewAPISyncPreviewDetectsRemovedChannels 验证：本地存在但源端未返回的渠道，
// 会被标记为 removed，且不会自动删除（同步预览只读）。
func TestBuildLocalNewAPISyncPreviewDetectsRemovedChannels(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	instance := LocalNewAPIInstance{ID: "inst-removed", Name: "Removed Detector"}
	insertImportedChannelForSyncPreviewTest(t, app, instance.ID, "ch-keep", "Alpha", "https://newapi.example.com", "1", "newapi")
	insertImportedChannelForSyncPreviewTest(t, app, instance.ID, "ch-gone", "Bravo", "https://newapi.example.org", "1", "newapi")

	// 源端只返回 ch-keep，ch-gone 应被识别为 removed。
	records := []map[string]interface{}{
		{"id": "ch-keep", "name": "Alpha", "base_url": "https://newapi.example.com", "status": "1"},
	}

	preview, err := app.buildLocalNewAPISyncPreview(context.Background(), instance, "sqlite", records)
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.RemovedCount != 1 {
		t.Fatalf("expected 1 removed channel, got %d (preview=%+v)", preview.RemovedCount, preview)
	}
	if preview.UnchangedCount != 1 {
		t.Fatalf("expected 1 unchanged (ch-keep), got %d", preview.UnchangedCount)
	}
	if preview.NewCount != 0 || preview.ChangedCount != 0 || preview.SkippedCount != 0 {
		t.Fatalf("expected no new/changed/skipped, got new=%d changed=%d skipped=%d",
			preview.NewCount, preview.ChangedCount, preview.SkippedCount)
	}

	removed := findPreviewItemBySourceID(preview.Items, "ch-gone")
	if removed == nil {
		t.Fatalf("expected removed item for ch-gone, items=%+v", preview.Items)
	}
	if removed.Action != "removed" {
		t.Fatalf("expected action=removed, got %q", removed.Action)
	}
	if removed.Name != "Bravo" {
		t.Fatalf("expected removed channel name Bravo, got %q", removed.Name)
	}
	if removed.Reason == "" {
		t.Fatal("expected non-empty reason for removed channel")
	}
}

// TestBuildLocalNewAPISyncPreviewClassifiesNewAndUnchanged 验证：源端返回的新渠道标记为 new，
// 字段一致的本地渠道标记为 unchanged。
func TestBuildLocalNewAPISyncPreviewClassifiesNewAndUnchanged(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	instance := LocalNewAPIInstance{ID: "inst-mixed", Name: "Mixed Classifier"}
	insertImportedChannelForSyncPreviewTest(t, app, instance.ID, "ch-same", "Stable", "https://newapi.example.com", "1", "newapi")

	records := []map[string]interface{}{
		{"id": "ch-same", "name": "Stable", "base_url": "https://newapi.example.com", "status": "1"},
		{"id": "ch-new", "name": "Fresh", "base_url": "https://newapi.example.net", "status": "1"},
	}

	preview, err := app.buildLocalNewAPISyncPreview(context.Background(), instance, "sqlite", records)
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.UnchangedCount != 1 {
		t.Fatalf("expected 1 unchanged, got %d", preview.UnchangedCount)
	}
	if preview.NewCount != 1 {
		t.Fatalf("expected 1 new, got %d", preview.NewCount)
	}
	if preview.RemovedCount != 0 || preview.ChangedCount != 0 || preview.SkippedCount != 0 {
		t.Fatalf("expected no removed/changed/skipped, got removed=%d changed=%d skipped=%d",
			preview.RemovedCount, preview.ChangedCount, preview.SkippedCount)
	}

	unchanged := findPreviewItemBySourceID(preview.Items, "ch-same")
	if unchanged == nil || unchanged.Action != "unchanged" {
		t.Fatalf("expected unchanged action for ch-same, got %+v", unchanged)
	}
	newItem := findPreviewItemBySourceID(preview.Items, "ch-new")
	if newItem == nil || newItem.Action != "new" {
		t.Fatalf("expected new action for ch-new, got %+v", newItem)
	}
}

// TestBuildLocalNewAPISyncPreviewDetectsChangedFields 验证：名称和状态变化的渠道标记为 changed，
// 且 ChangedFields 正确列出变化的字段。
func TestBuildLocalNewAPISyncPreviewDetectsChangedFields(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	instance := LocalNewAPIInstance{ID: "inst-changed", Name: "Change Detector"}
	insertImportedChannelForSyncPreviewTest(t, app, instance.ID, "ch-change", "Old Name", "https://newapi.example.com", "1", "newapi")

	records := []map[string]interface{}{
		{"id": "ch-change", "name": "New Name", "base_url": "https://newapi.example.com", "status": "2"},
	}

	preview, err := app.buildLocalNewAPISyncPreview(context.Background(), instance, "sqlite", records)
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.ChangedCount != 1 {
		t.Fatalf("expected 1 changed, got %d (preview=%+v)", preview.ChangedCount, preview)
	}

	changed := findPreviewItemBySourceID(preview.Items, "ch-change")
	if changed == nil {
		t.Fatalf("expected changed item for ch-change, items=%+v", preview.Items)
	}
	if changed.Action != "changed" {
		t.Fatalf("expected action=changed, got %q", changed.Action)
	}
	if len(changed.ChangedFields) < 2 {
		t.Fatalf("expected at least 2 changed fields (名称, 状态), got %v", changed.ChangedFields)
	}
	hasName := false
	hasStatus := false
	for _, f := range changed.ChangedFields {
		if f == "名称" {
			hasName = true
		}
		if f == "状态" {
			hasStatus = true
		}
	}
	if !hasName || !hasStatus {
		t.Fatalf("expected 名称 and 状态 in changed fields, got %v", changed.ChangedFields)
	}
}

// TestBuildLocalNewAPISyncPreviewSkipsExcludedRelaySites 验证：命中排除规则的纯路由站渠道
// 会被标记为 skipped，不会进入 new/changed 流程。
func TestBuildLocalNewAPISyncPreviewSkipsExcludedRelaySites(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	instance := LocalNewAPIInstance{ID: "inst-skip", Name: "Skip Detector"}

	records := []map[string]interface{}{
		{"id": "ch-skip", "name": "TokenRouter Relay", "base_url": "https://example.com", "status": "1"},
	}

	preview, err := app.buildLocalNewAPISyncPreview(context.Background(), instance, "sqlite", records)
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.SkippedCount != 1 {
		t.Fatalf("expected 1 skipped, got %d (preview=%+v)", preview.SkippedCount, preview)
	}
	if preview.NewCount != 0 {
		t.Fatalf("expected 0 new (should be skipped), got %d", preview.NewCount)
	}
	skipped := findPreviewItemBySourceID(preview.Items, "ch-skip")
	if skipped == nil || skipped.Action != "skipped" {
		t.Fatalf("expected skipped action for ch-skip, got %+v", skipped)
	}
}
