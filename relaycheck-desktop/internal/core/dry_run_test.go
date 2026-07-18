package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDryRunAllDueUsesServerPlanAndDoesNotExposeCredentials(t *testing.T) {
	app := newTestApp(t)
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "dry-run-ready", name: "Ready account", supportsCheckin: 1,
		cookie: "session=TOP_SECRET_COOKIE", apiKey: "TOP_SECRET_API_KEY", updatedAt: "2026-07-18T02:00:00Z",
	})
	insertCheckinPlanFixture(t, app, checkinPlanFixture{
		id: "dry-run-missing", name: "Missing account", supportsCheckin: 1, updatedAt: "2026-07-18T01:00:00Z",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(`{"type":"checkin","scope":{"kind":"all_due"}}`))
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)
	if preview.TotalAccounts != 2 || preview.WillRun != 1 || preview.Skipped != 1 || preview.PreviewID == "" {
		t.Fatalf("unexpected all_due preview: %#v", preview)
	}
	for _, secret := range []string{"TOP_SECRET_COOKIE", "TOP_SECRET_API_KEY", "session="} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("dry-run response exposed %q: %s", secret, rec.Body.String())
		}
	}
}

func TestDryRunAllDueRequiresScopeAndRejectsThe201stAccountWith409(t *testing.T) {
	t.Run("scope required", func(t *testing.T) {
		app := newTestApp(t)
		req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(`{"type":"checkin"}`))
		rec := httptest.NewRecorder()
		app.handleDryRun(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing scope status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("over limit", func(t *testing.T) {
		app := newTestApp(t)
		siteID := "dry-run-over-limit-site"
		stamp := "2026-07-18T01:00:00Z"
		if _, err := app.db.Exec(`
			INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
			VALUES (?, 'Over limit', 'https://dry-run-over-limit.example', 'newapi', 'healthy', 1, ?, ?)
		`, siteID, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		tx, err := app.db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i <= maxDryRunAccounts; i++ {
			id := fmt.Sprintf("dry-run-over-%03d", i)
			if _, err := tx.Exec(`
				INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
				VALUES (?, ?, ?, 'api_key', 'valid', ?, ?)
			`, id, siteID, id, stamp, stamp); err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(`{"type":"checkin","scope":{"kind":"all_due"}}`))
		rec := httptest.NewRecorder()
		app.handleDryRun(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("over-limit status = %d, want 409: %s", rec.Code, rec.Body.String())
		}
		if app.checkinPlanStore.count() != 0 {
			t.Fatal("over-limit handler stored a partial preview")
		}
	})
}

// insertDryRunAccount 插入一条测试账号 + 关联站点，用于 dry-run 预览测试。
func insertDryRunAccount(t *testing.T, app *App, id, name, siteName, loginStatus, authType string, supportsCheckin int) {
	t.Helper()
	siteID := newID()
	_, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, ?, 'https://dryrun.example', 'newapi', 'healthy', ?, ?, ?)
	`, siteID, siteName, supportsCheckin, now(), now())
	if err != nil {
		t.Fatalf("insert site %s: %v", siteName, err)
	}
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, login_status, auth_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, siteID, name, loginStatus, authType, now(), now())
	if err != nil {
		t.Fatalf("insert account %s: %v", id, err)
	}
}

func decodeDryRunResponse(t *testing.T, rec *httptest.ResponseRecorder) DryRunPreview {
	t.Helper()
	var response struct {
		OK   bool          `json:"ok"`
		Data DryRunPreview `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rec.Body.String())
	}
	return response.Data
}

// TestDryRunRejectsExceedingAccountLimit 验证：超过 200 个账号上限的请求会被拒绝，
// 符合 project_memory 中 Dry run 最多 200 条的硬约束。
func TestDryRunRejectsExceedingAccountLimit(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	accountIDs := make([]string, 201)
	for i := range accountIDs {
		accountIDs[i] = newID()
	}
	body := `{"type":"test","accountIds":["` + strings.Join(accountIDs, `","`) + `"]}`

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201 accounts, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "200") {
		t.Fatalf("expected error message to mention limit 200, got: %s", rec.Body.String())
	}
}

// TestDryRunRejectsMissingTypeOrAccountIds 验证：缺少 type 或 accountIds 的请求被拒绝。
func TestDryRunRejectsMissingTypeOrAccountIds(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	cases := []struct {
		name string
		body string
	}{
		{"missing type", `{"accountIds":["a"]}`},
		{"missing accountIds", `{"type":"checkin"}`},
		{"empty body", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(tc.body))
			req.Header.Set("content-type", "application/json")
			rec := httptest.NewRecorder()
			app.handleDryRun(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestDryRunRejectsWrongMethod 验证：非 POST 请求被拒绝。
func TestDryRunRejectsWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/dry-run", nil)
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// TestDryRunClassifiesCheckinActions verifies that all_due uses decrypted
// authentication facts instead of login_status/auth_type heuristics.
func TestDryRunClassifiesCheckinActions(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idOK := "acc-ok"
	idUnsupported := "acc-unsupported"
	idExpired := "acc-expired"
	idNoCookie := "acc-no-cookie"

	insertDryRunAccount(t, app, idOK, "OK Account", "OK Site", "valid", "cookie", 1)
	insertDryRunAccount(t, app, idUnsupported, "Unsupported Account", "No Checkin Site", "valid", "api_key", 0)
	insertDryRunAccount(t, app, idExpired, "Expired Account", "Expired Site", "expired", "cookie", 1)
	// skip_no_cookie 分支要求 authType=cookie 且 loginStatus 既非 expired 也非 logged_out，
	// 但又不是 valid——用 "pending" 模拟 cookie 未保存的状态。
	insertDryRunAccount(t, app, idNoCookie, "No Cookie Account", "Cookie Site", "pending", "cookie", 1)
	apiKeyEncrypted, err := app.encryptText("saved-api-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE channel_accounts SET api_key_encrypted=? WHERE id=?`, apiKeyEncrypted, idOK); err != nil {
		t.Fatal(err)
	}

	body := `{"type":"checkin","scope":{"kind":"all_due"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if preview.TotalAccounts != 4 {
		t.Fatalf("expected total=4, got %d", preview.TotalAccounts)
	}
	if preview.WillRun != 1 {
		t.Fatalf("expected willRun=1, got %d", preview.WillRun)
	}
	if preview.Skipped != 3 {
		t.Fatalf("expected skipped=3, got %d", preview.Skipped)
	}
	if len(preview.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(preview.Items))
	}

	findItem := func(id string) *DryRunPreviewItem {
		for i := range preview.Items {
			if preview.Items[i].AccountID == id {
				return &preview.Items[i]
			}
		}
		return nil
	}

	if item := findItem(idOK); item == nil || item.Action != "will_run" {
		t.Fatalf("expected idOK will_run, got %+v", item)
	}
	if item := findItem(idUnsupported); item == nil || item.Action != "skip_unsupported" {
		t.Fatalf("expected idUnsupported skip_unsupported, got %+v", item)
	}
	if item := findItem(idExpired); item == nil || item.Action != "skip_missing_credentials" {
		t.Fatalf("expected idExpired skip_missing_credentials, got %+v", item)
	}
	if item := findItem(idNoCookie); item == nil || item.Action != "skip_missing_credentials" {
		t.Fatalf("expected idNoCookie skip_missing_credentials, got %+v", item)
	}
}

// TestDryRunSkipsNotFoundAccounts verifies caller-supplied IDs cannot expand
// the server-owned all_due selection.
func TestDryRunSkipsNotFoundAccounts(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idReal := "acc-real"
	idGhost := "acc-ghost"
	insertDryRunAccount(t, app, idReal, "Real Account", "Real Site", "valid", "api_key", 1)
	apiKeyEncrypted, err := app.encryptText("saved-api-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE channel_accounts SET api_key_encrypted=? WHERE id=?`, apiKeyEncrypted, idReal); err != nil {
		t.Fatal(err)
	}

	body := `{"type":"checkin","scope":{"kind":"all_due"},"accountIds":["` + idReal + `","` + idGhost + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if preview.TotalAccounts != 1 {
		t.Fatalf("expected server-owned total=1, got %d", preview.TotalAccounts)
	}
	if preview.WillRun != 1 {
		t.Fatalf("expected willRun=1, got %d", preview.WillRun)
	}
	if preview.Skipped != 0 {
		t.Fatalf("expected skipped=0, got %d", preview.Skipped)
	}

	var ghost *DryRunPreviewItem
	for i := range preview.Items {
		if preview.Items[i].AccountID == idGhost {
			ghost = &preview.Items[i]
		}
	}
	if ghost != nil {
		t.Fatalf("caller-supplied ghost account expanded all_due selection: %#v", ghost)
	}
}

// TestDryRunHandlesUnknownType 验证：未知操作类型的账号被标记为 skip_unknown_type。
func TestDryRunHandlesUnknownType(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idTest := "acc-unknown-type"
	insertDryRunAccount(t, app, idTest, "Test Account", "Test Site", "valid", "api_key", 1)

	body := `{"type":"unknown_op","accountIds":["` + idTest + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(preview.Items))
	}
	if preview.Items[0].Action != "skip_unknown_type" {
		t.Fatalf("expected skip_unknown_type, got %q", preview.Items[0].Action)
	}
	if preview.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", preview.Skipped)
	}
}

// TestDryRunCheckinSkipsLoggedOutStatus verifies loginStatus alone does not
// suppress an account that has a usable local authentication path.
func TestDryRunCheckinSkipsLoggedOutStatus(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	id := "acc-logged-out"
	insertDryRunAccount(t, app, id, "Logged Out Account", "LoggedOut Site", "logged_out", "api_key", 1)
	apiKeyEncrypted, err := app.encryptText("saved-api-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE channel_accounts SET api_key_encrypted=? WHERE id=?`, apiKeyEncrypted, id); err != nil {
		t.Fatal(err)
	}

	body := `{"type":"checkin","scope":{"kind":"all_due"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if len(preview.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(preview.Items))
	}
	if preview.Items[0].Action != "will_run" {
		t.Fatalf("expected will_run for logged_out with API key, got %q", preview.Items[0].Action)
	}
	if preview.Skipped != 0 || preview.WillRun != 1 {
		t.Fatalf("expected skipped=0 willRun=1, got skipped=%d willRun=%d", preview.Skipped, preview.WillRun)
	}
}

// TestDryRunTestAndIdentifyTypesAlwaysWillRun 验证：test 和 identify 类型对存在的账号总是 will_run。
func TestDryRunTestAndIdentifyTypesAlwaysWillRun(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idTest := "acc-test-type"
	idIdentify := "acc-identify-type"
	insertDryRunAccount(t, app, idTest, "Test Account", "Test Site", "valid", "api_key", 0)
	insertDryRunAccount(t, app, idIdentify, "Identify Account", "Identify Site", "expired", "cookie", 0)

	for _, opType := range []string{"test", "identify"} {
		t.Run(opType, func(t *testing.T) {
			body := `{"type":"` + opType + `","accountIds":["` + idTest + `","` + idIdentify + `"]}`
			req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
			req.Header.Set("content-type", "application/json")
			rec := httptest.NewRecorder()
			app.handleDryRun(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			preview := decodeDryRunResponse(t, rec)
			if preview.WillRun != 2 {
				t.Fatalf("expected willRun=2 for %s, got %d", opType, preview.WillRun)
			}
			if preview.Skipped != 0 {
				t.Fatalf("expected skipped=0 for %s, got %d", opType, preview.Skipped)
			}
		})
	}
}

// TestDryRunPreservesRequestOrder verifies all_due preserves the due selector's
// updated_at ordering; caller accountIds do not define execution order.
func TestDryRunPreservesRequestOrder(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idA := "acc-order-a"
	idB := "acc-order-b"
	idC := "acc-order-c"
	insertDryRunAccount(t, app, idA, "A", "Site A", "valid", "api_key", 1)
	insertDryRunAccount(t, app, idB, "B", "Site B", "valid", "api_key", 1)
	insertDryRunAccount(t, app, idC, "C", "Site C", "valid", "api_key", 1)
	apiKeyEncrypted, err := app.encryptText("saved-api-key")
	if err != nil {
		t.Fatal(err)
	}
	for id, updatedAt := range map[string]string{
		idA: "2026-07-18T02:00:00Z",
		idB: "2026-07-18T01:00:00Z",
		idC: "2026-07-18T03:00:00Z",
	} {
		if _, err := app.db.Exec(`UPDATE channel_accounts SET api_key_encrypted=?, updated_at=? WHERE id=?`, apiKeyEncrypted, updatedAt, id); err != nil {
			t.Fatal(err)
		}
	}

	body := `{"type":"checkin","scope":{"kind":"all_due"},"accountIds":["` + idB + `","` + idA + `","` + idC + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if len(preview.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(preview.Items))
	}
	if preview.Items[0].AccountID != idC {
		t.Fatalf("expected first item idC, got %q", preview.Items[0].AccountID)
	}
	if preview.Items[1].AccountID != idA {
		t.Fatalf("expected second item idA, got %q", preview.Items[1].AccountID)
	}
	if preview.Items[2].AccountID != idB {
		t.Fatalf("expected third item idB, got %q", preview.Items[2].AccountID)
	}
}
