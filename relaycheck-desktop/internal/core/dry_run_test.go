package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	body := `{"type":"checkin","accountIds":["` + strings.Join(accountIDs, `","`) + `"]}`

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

// TestDryRunClassifiesCheckinActions 验证：checkin 类型的 dry-run 正确分类账号为
// will_run / skip_unsupported / skip_expired / skip_no_cookie。
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

	body := `{"type":"checkin","accountIds":["` + strings.Join([]string{idOK, idUnsupported, idExpired, idNoCookie}, `","`) + `"]}`
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
	if item := findItem(idExpired); item == nil || item.Action != "skip_expired" {
		t.Fatalf("expected idExpired skip_expired, got %+v", item)
	}
	if item := findItem(idNoCookie); item == nil || item.Action != "skip_no_cookie" {
		t.Fatalf("expected idNoCookie skip_no_cookie, got %+v", item)
	}
}

// TestDryRunSkipsNotFoundAccounts 验证：请求中存在但数据库中不存在的账号 ID 被标记为 skip_not_found。
func TestDryRunSkipsNotFoundAccounts(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idReal := "acc-real"
	idGhost := "acc-ghost"
	insertDryRunAccount(t, app, idReal, "Real Account", "Real Site", "valid", "api_key", 1)

	body := `{"type":"checkin","accountIds":["` + idReal + `","` + idGhost + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/dry-run", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleDryRun(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	preview := decodeDryRunResponse(t, rec)

	if preview.TotalAccounts != 2 {
		t.Fatalf("expected total=2, got %d", preview.TotalAccounts)
	}
	if preview.WillRun != 1 {
		t.Fatalf("expected willRun=1, got %d", preview.WillRun)
	}
	if preview.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", preview.Skipped)
	}

	var ghost *DryRunPreviewItem
	for i := range preview.Items {
		if preview.Items[i].AccountID == idGhost {
			ghost = &preview.Items[i]
		}
	}
	if ghost == nil {
		t.Fatal("expected ghost account in items")
	}
	if ghost.Action != "skip_not_found" {
		t.Fatalf("expected skip_not_found, got %q", ghost.Action)
	}
	if ghost.AccountName != "未知" {
		t.Fatalf("expected 未知, got %q", ghost.AccountName)
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

// TestDryRunCheckinSkipsLoggedOutStatus verifies that loginStatus "logged_out"
// hits the same skip_expired branch as "expired" in the checkin path.
func TestDryRunCheckinSkipsLoggedOutStatus(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	id := "acc-logged-out"
	insertDryRunAccount(t, app, id, "Logged Out Account", "LoggedOut Site", "logged_out", "api_key", 1)

	body := `{"type":"checkin","accountIds":["` + id + `"]}`
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
	if preview.Items[0].Action != "skip_expired" {
		t.Fatalf("expected skip_expired for logged_out, got %q", preview.Items[0].Action)
	}
	if preview.Skipped != 1 || preview.WillRun != 0 {
		t.Fatalf("expected skipped=1 willRun=0, got skipped=%d willRun=%d", preview.Skipped, preview.WillRun)
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

// TestDryRunPreservesRequestOrder 验证：preview.items 的顺序与请求中 accountIds 顺序一致，
// 而非数据库返回顺序。
func TestDryRunPreservesRequestOrder(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	idA := "acc-order-a"
	idB := "acc-order-b"
	idC := "acc-order-c"
	insertDryRunAccount(t, app, idA, "A", "Site A", "valid", "api_key", 1)
	insertDryRunAccount(t, app, idB, "B", "Site B", "valid", "api_key", 1)
	insertDryRunAccount(t, app, idC, "C", "Site C", "valid", "api_key", 1)

	body := `{"type":"checkin","accountIds":["` + idC + `","` + idA + `","` + idB + `"]}`
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
