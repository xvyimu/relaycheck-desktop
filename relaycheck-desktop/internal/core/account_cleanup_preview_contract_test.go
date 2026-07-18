package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type cleanupPreviewContractResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Matched   int    `json:"matched"`
		Deleted   int    `json:"deleted"`
		PreviewID string `json:"previewId"`
		ExpiresAt string `json:"expiresAt"`
		Items     []struct {
			AccountID string `json:"accountId"`
		} `json:"items"`
	} `json:"data"`
	Error      string `json:"error"`
	ErrorClass string `json:"errorClass"`
}

type unsupportedCleanupFixture struct {
	siteID    string
	accountID string
}

// seedUnsupportedCleanupFixture 创建一个可清理账号及其签到日志、余额快照，供破坏性契约测试复用。
func seedUnsupportedCleanupFixture(t *testing.T, app *App, label, updatedAt string) unsupportedCleanupFixture {
	t.Helper()
	fixture := unsupportedCleanupFixture{
		siteID:    newID(),
		accountID: newID(),
	}
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, ?, ?, 'newapi', 'healthy', 0, ?, ?)
	`, fixture.siteID, "Unsupported "+label, "https://"+strings.ToLower(label)+".example", updatedAt, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, last_checkin_status, created_at, updated_at)
		VALUES (?, ?, ?, 'cookie', 'valid', 'unsupported', ?, ?)
	`, fixture.accountID, fixture.siteID, "Account "+label, updatedAt, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO checkin_logs (id, account_id, upstream_site_id, status, started_at, finished_at)
		VALUES (?, ?, ?, 'unsupported', ?, ?)
	`, newID(), fixture.accountID, fixture.siteID, updatedAt, updatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO balance_snapshots (id, account_id, upstream_site_id, unit, created_at)
		VALUES (?, ?, ?, 'quota', ?)
	`, newID(), fixture.accountID, fixture.siteID, updatedAt); err != nil {
		t.Fatal(err)
	}
	return fixture
}

// callUnsupportedCleanup 调用账号清理 handler，并按公共 JSON envelope 解码响应。
func callUnsupportedCleanup(t *testing.T, app *App, body string) (*httptest.ResponseRecorder, cleanupPreviewContractResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/delete-unsupported-checkins", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDeleteUnsupportedCheckinAccounts(rec, req)

	var response cleanupPreviewContractResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode cleanup response: %v; body=%s", err, rec.Body.String())
	}
	return rec, response
}

// assertCleanupFixtureRows 验证账号及两类关联历史是否保持预期数量，防止失败路径发生部分删除。
func assertCleanupFixtureRows(t *testing.T, app *App, fixture unsupportedCleanupFixture, want int) {
	t.Helper()
	queries := []struct {
		name  string
		query string
	}{
		{name: "account", query: `SELECT COUNT(*) FROM channel_accounts WHERE id=?`},
		{name: "checkin log", query: `SELECT COUNT(*) FROM checkin_logs WHERE account_id=?`},
		{name: "balance snapshot", query: `SELECT COUNT(*) FROM balance_snapshots WHERE account_id=?`},
	}
	for _, item := range queries {
		var got int
		if err := app.db.QueryRow(item.query, fixture.accountID).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", item.name, err)
		}
		if got != want {
			t.Fatalf("%s rows = %d, want %d", item.name, got, want)
		}
	}
}

// createUnsupportedCleanupPreview 创建一次清理预览并返回只能消费一次的 previewId。
func createUnsupportedCleanupPreview(t *testing.T, app *App, limit int) cleanupPreviewContractResponse {
	t.Helper()
	rec, response := callUnsupportedCleanup(t, app, fmt.Sprintf(`{"dryRun":true,"limit":%d,"includeLastUnsupported":false}`, limit))
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if response.Data.PreviewID == "" || response.Data.ExpiresAt == "" {
		t.Fatalf("preview must freeze candidates with previewId/expiresAt: %#v", response.Data)
	}
	return response
}

// TestUnsupportedCleanupPreviewDoesNotExpandAfterHigherSortedCandidateAppears 验证新增高排序候选不会替换冻结集合。
func TestUnsupportedCleanupPreviewDoesNotExpandAfterHigherSortedCandidateAppears(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	frozen := seedUnsupportedCleanupFixture(t, app, "Frozen", "2026-07-18T01:00:00Z")
	preview := createUnsupportedCleanupPreview(t, app, 1)
	if len(preview.Data.Items) != 1 || preview.Data.Items[0].AccountID != frozen.accountID {
		t.Fatalf("unexpected frozen preview: %#v", preview.Data.Items)
	}

	newer := seedUnsupportedCleanupFixture(t, app, "Newer", "2026-07-18T02:00:00Z")
	rec, response := callUnsupportedCleanup(t, app, fmt.Sprintf(`{"dryRun":false,"previewId":%q}`, preview.Data.PreviewID))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if response.Data.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", response.Data.Deleted)
	}
	assertCleanupFixtureRows(t, app, frozen, 0)
	assertCleanupFixtureRows(t, app, newer, 1)
}

// TestUnsupportedCleanupPreviewRejectsCandidateStateDriftWithoutDeleting 验证候选漂移时三张表保持不变。
func TestUnsupportedCleanupPreviewRejectsCandidateStateDriftWithoutDeleting(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	fixture := seedUnsupportedCleanupFixture(t, app, "Drift", "2026-07-18T01:00:00Z")
	preview := createUnsupportedCleanupPreview(t, app, 1)
	if _, err := app.db.Exec(`UPDATE upstream_sites SET supports_checkin=1, updated_at=? WHERE id=?`, "2026-07-18T02:00:00Z", fixture.siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE channel_accounts SET last_checkin_status='success', updated_at=? WHERE id=?`, "2026-07-18T02:00:00Z", fixture.accountID); err != nil {
		t.Fatal(err)
	}

	rec, _ := callUnsupportedCleanup(t, app, fmt.Sprintf(`{"dryRun":false,"previewId":%q}`, preview.Data.PreviewID))
	if rec.Code != http.StatusConflict {
		t.Fatalf("drift status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	assertCleanupFixtureRows(t, app, fixture, 1)
}

// TestUnsupportedCleanupPreviewIDFailsClosed 验证缺失、不可用和重放 token 均安全失败。
func TestUnsupportedCleanupPreviewIDFailsClosed(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		app := newTestApp(t)
		defer app.Close()
		fixture := seedUnsupportedCleanupFixture(t, app, "Missing", "2026-07-18T01:00:00Z")

		rec, _ := callUnsupportedCleanup(t, app, `{"dryRun":false}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing previewId status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
		assertCleanupFixtureRows(t, app, fixture, 1)
	})

	t.Run("expired or unavailable", func(t *testing.T) {
		app := newTestApp(t)
		defer app.Close()
		fixture := seedUnsupportedCleanupFixture(t, app, "Expired", "2026-07-18T01:00:00Z")

		rec, _ := callUnsupportedCleanup(t, app, `{"dryRun":false,"previewId":"expired-preview-id"}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("expired previewId status = %d, want 409; body=%s", rec.Code, rec.Body.String())
		}
		assertCleanupFixtureRows(t, app, fixture, 1)
	})

	t.Run("replayed", func(t *testing.T) {
		app := newTestApp(t)
		defer app.Close()
		fixture := seedUnsupportedCleanupFixture(t, app, "Replay", "2026-07-18T01:00:00Z")
		preview := createUnsupportedCleanupPreview(t, app, 1)
		body := fmt.Sprintf(`{"dryRun":false,"previewId":%q}`, preview.Data.PreviewID)

		first, _ := callUnsupportedCleanup(t, app, body)
		if first.Code != http.StatusOK {
			t.Fatalf("first claim status = %d, body=%s", first.Code, first.Body.String())
		}
		assertCleanupFixtureRows(t, app, fixture, 0)

		second, _ := callUnsupportedCleanup(t, app, body)
		if second.Code != http.StatusConflict {
			t.Fatalf("replayed previewId status = %d, want 409; body=%s", second.Code, second.Body.String())
		}
	})
}

// TestUnsupportedCleanupEmptyBodyDoesNotDelete 验证空请求体不能利用布尔零值触发删除。
func TestUnsupportedCleanupEmptyBodyDoesNotDelete(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	fixture := seedUnsupportedCleanupFixture(t, app, "Empty", "2026-07-18T01:00:00Z")

	rec, _ := callUnsupportedCleanup(t, app, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	assertCleanupFixtureRows(t, app, fixture, 1)
}

// TestUnsupportedCleanupConfirmRejectsReselectionFields 验证确认阶段不能重交 limit 或过滤范围。
func TestUnsupportedCleanupConfirmRejectsReselectionFields(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	fixture := seedUnsupportedCleanupFixture(t, app, "Reselection", "2026-07-18T01:00:00Z")
	preview := createUnsupportedCleanupPreview(t, app, 1)

	rec, _ := callUnsupportedCleanup(t, app, fmt.Sprintf(
		`{"previewId":%q,"limit":10,"includeLastUnsupported":true}`,
		preview.Data.PreviewID,
	))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reselection status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	assertCleanupFixtureRows(t, app, fixture, 1)
}

// TestUnsupportedCleanupPreviewStoreExpiresAndReleasesCapacity 验证过期 token 不可消费且会释放有界容量。
func TestUnsupportedCleanupPreviewStoreExpiresAndReleasesCapacity(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	seedUnsupportedCleanupFixture(t, app, "Capacity", "2026-07-18T01:00:00Z")

	clock := time.Date(2026, 7, 18, 2, 0, 0, 0, time.UTC)
	app.accountCleanup.now = func() time.Time { return clock }
	previewIDs := make([]string, 0, maxPendingAccountCleanupPreviews)
	for index := 0; index < maxPendingAccountCleanupPreviews; index++ {
		preview, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false)
		if err != nil {
			t.Fatalf("preview %d: %v", index, err)
		}
		previewIDs = append(previewIDs, preview.PreviewID)
	}
	if _, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false); !errors.Is(err, errAccountCleanupPreviewCapacity) {
		t.Fatalf("overflow error = %v, want capacity error", err)
	}

	clock = clock.Add(accountCleanupPreviewTTL)
	if _, err := app.confirmUnsupportedCheckinAccounts(context.Background(), previewIDs[0]); !errors.Is(err, errAccountCleanupPreviewUnavailable) {
		t.Fatalf("expired claim error = %v, want unavailable", err)
	}
	if preview, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false); err != nil || preview.PreviewID == "" {
		t.Fatalf("preview after expiry = %#v, err=%v", preview, err)
	}
}

// TestUnsupportedCleanupConcurrentConfirmAllowsOneWinner 验证同一 previewId 的并发确认最多成功一次。
func TestUnsupportedCleanupConcurrentConfirmAllowsOneWinner(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	fixture := seedUnsupportedCleanupFixture(t, app, "Concurrent", "2026-07-18T01:00:00Z")
	preview, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}

	errorsByWorker := make([]error, 2)
	var wait sync.WaitGroup
	for index := range errorsByWorker {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			_, errorsByWorker[worker] = app.confirmUnsupportedCheckinAccounts(context.Background(), preview.PreviewID)
		}(index)
	}
	wait.Wait()

	successes := 0
	unavailable := 0
	for _, confirmErr := range errorsByWorker {
		if confirmErr == nil {
			successes++
		} else if errors.Is(confirmErr, errAccountCleanupPreviewUnavailable) {
			unavailable++
		}
	}
	if successes != 1 || unavailable != 1 {
		t.Fatalf("confirm outcomes = %#v, want one success and one unavailable", errorsByWorker)
	}
	assertCleanupFixtureRows(t, app, fixture, 0)
}

// TestUnsupportedCleanupTransactionRollsBackRelatedDeletes 验证关联数据删除中途失败时账号与历史记录全部回滚。
func TestUnsupportedCleanupTransactionRollsBackRelatedDeletes(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	fixture := seedUnsupportedCleanupFixture(t, app, "Rollback", "2026-07-18T01:00:00Z")
	preview, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		CREATE TRIGGER fail_cleanup_balance_delete
		BEFORE DELETE ON balance_snapshots
		BEGIN
			SELECT RAISE(ABORT, 'forced cleanup rollback');
		END
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := app.confirmUnsupportedCheckinAccounts(context.Background(), preview.PreviewID); err == nil {
		t.Fatal("forced balance delete failure must abort cleanup")
	}
	assertCleanupFixtureRows(t, app, fixture, 1)
}

// TestUnsupportedCleanupRebindUsesNewDatabaseAndClearsPreviews 验证恢复重绑后旧 token 不会跨数据集生效。
func TestUnsupportedCleanupRebindUsesNewDatabaseAndClearsPreviews(t *testing.T) {
	oldApp := newTestApp(t)
	defer oldApp.Close()
	seedUnsupportedCleanupFixture(t, oldApp, "OldDB", "2026-07-18T01:00:00Z")
	oldPreview, err := oldApp.previewUnsupportedCheckinAccounts(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}

	newApp := newTestApp(t)
	defer newApp.Close()
	newFixture := seedUnsupportedCleanupFixture(t, newApp, "NewDB", "2026-07-18T02:00:00Z")
	oldApp.accountCleanup.RebindDB(newApp.db)

	if _, err := oldApp.confirmUnsupportedCheckinAccounts(context.Background(), oldPreview.PreviewID); !errors.Is(err, errAccountCleanupPreviewUnavailable) {
		t.Fatalf("old preview after rebind error = %v, want unavailable", err)
	}
	newPreview, err := oldApp.previewUnsupportedCheckinAccounts(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(newPreview.Items) != 1 || newPreview.Items[0].AccountID != newFixture.accountID {
		t.Fatalf("preview after rebind = %#v, want new database account", newPreview.Items)
	}
}

// TestUnsupportedCleanupCapacityUsesRateLimitContract 验证预览容量耗尽时返回统一的 429 契约。
func TestUnsupportedCleanupCapacityUsesRateLimitContract(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	seedUnsupportedCleanupFixture(t, app, "RateLimit", "2026-07-18T01:00:00Z")
	for index := 0; index < maxPendingAccountCleanupPreviews; index++ {
		if _, err := app.previewUnsupportedCheckinAccounts(context.Background(), 1, false); err != nil {
			t.Fatalf("preview %d: %v", index, err)
		}
	}

	rec, response := callUnsupportedCleanup(t, app, `{"dryRun":true,"limit":1,"includeLastUnsupported":false}`)
	if rec.Code != http.StatusTooManyRequests || response.ErrorClass != "rate_limited" {
		t.Fatalf("capacity response = status %d class %q, body=%s", rec.Code, response.ErrorClass, rec.Body.String())
	}
}

// TestUnsupportedCleanupInternalErrorUsesStablePublicMessage 验证数据库错误不会进入公共响应。
func TestUnsupportedCleanupInternalErrorUsesStablePublicMessage(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	seedUnsupportedCleanupFixture(t, app, "PublicError", "2026-07-18T01:00:00Z")
	preview := createUnsupportedCleanupPreview(t, app, 1)
	if _, err := app.db.Exec(`
		CREATE TRIGGER fail_cleanup_public_error
		BEFORE DELETE ON balance_snapshots
		BEGIN
			SELECT RAISE(ABORT, 'TOP_SECRET cleanup database failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	rec, response := callUnsupportedCleanup(t, app, fmt.Sprintf(`{"previewId":%q}`, preview.Data.PreviewID))
	if rec.Code != http.StatusInternalServerError || response.Error != "服务暂时不可用，请稍后重试。" {
		t.Fatalf("internal response = status %d error %q", rec.Code, response.Error)
	}
	if strings.Contains(rec.Body.String(), "TOP_SECRET") {
		t.Fatalf("internal database error leaked: %s", rec.Body.String())
	}
}
