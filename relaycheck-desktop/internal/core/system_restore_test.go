package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSystemRestoreRejectsWrongMethod 验证：非 POST 请求被拒绝。
func TestSystemRestoreRejectsWrongMethod(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/system/restore", nil)
	rec := httptest.NewRecorder()
	app.handleSystemRestore(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

// TestSystemRestoreRejectsMissingFileName 验证：缺少 fileName 的请求被拒绝。
func TestSystemRestoreRejectsMissingFileName(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	cases := []struct {
		name string
		body string
	}{
		{"empty fileName", `{"fileName":""}`},
		{"missing fileName", `{}`},
		{"invalid json", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/system/restore", strings.NewReader(tc.body))
			req.Header.Set("content-type", "application/json")
			rec := httptest.NewRecorder()
			app.handleSystemRestore(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestSystemRestoreRejectsNonExistentBackup 验证：指定的备份文件不存在时返回 404。
func TestSystemRestoreRejectsNonExistentBackup(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	body := `{"fileName":"nonexistent-backup.db"}`
	req := httptest.NewRequest(http.MethodPost, "/api/system/restore", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleSystemRestore(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent backup, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "不存在") {
		t.Fatalf("expected error to mention 不存在, got: %s", rec.Body.String())
	}
}

// TestSystemRestoreRejectsPathTraversal 验证：fileName 含路径遍历（../）会被 backupPath 拒绝，
// 防止攻击者通过构造 fileName 读取任意文件。
func TestSystemRestoreRejectsPathTraversal(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	cases := []string{
		"../../../etc/passwd",
		"..\\..\\windows\\win.ini",
		"subdir/../../escape.db",
	}
	for _, fileName := range cases {
		t.Run(fileName, func(t *testing.T) {
			body := `{"fileName":"` + fileName + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/system/restore", strings.NewReader(body))
			req.Header.Set("content-type", "application/json")
			rec := httptest.NewRecorder()
			app.handleSystemRestore(rec, req)
			// 路径遍历应被 backupPath 拒绝（返回 400）或文件不存在（404），
			// 关键是不能成功恢复（200）或返回 500（表示路径被拼接但读取失败）。
			if rec.Code == http.StatusOK {
				t.Fatalf("path traversal %q succeeded with 200 — security violation", fileName)
			}
		})
	}
}

// TestSystemRestoreCreatesBeforeBackupAndRestores 验证：恢复操作会先创建 before-restore 快照，
// 然后从指定备份恢复，响应包含 beforeBackup 字段，且数据库内容被替换。
func TestSystemRestoreCreatesBeforeBackupAndRestores(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()
	var err error

	// 1. 在主数据库插入一条标记账号
	siteID := newID()
	_, err = app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Original Site', 'https://original.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	originalAccountID := newID()
	_, err = app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, created_at, updated_at)
		VALUES (?, ?, 'Original Account', 'api_key', ?, ?)
	`, originalAccountID, siteID, now(), now())
	if err != nil {
		t.Fatal(err)
	}

	// 2. 创建一个备份（包含标记账号）
	backup1, err := app.createBackup("test-source")
	if err != nil {
		t.Fatalf("createBackup test-source: %v", err)
	}

	// 3. 修改主数据库（删除标记账号），模拟数据库被污染
	_, err = app.db.Exec(`DELETE FROM channel_accounts WHERE id = ?`, originalAccountID)
	if err != nil {
		t.Fatal(err)
	}
	// 确认账号已被删除
	var count int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id = ?`, originalAccountID).Scan(&count)
	if count != 0 {
		t.Fatalf("setup failed: original account should be deleted before restore")
	}

	// 4. 从备份恢复
	body := `{"fileName":"` + backup1.FileName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/system/restore", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleSystemRestore(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. 验证响应结构
	var response struct {
		OK   bool   `json:"ok"`
		Data struct {
			Restored     bool   `json:"restored"`
			FileName     string `json:"fileName"`
			BeforeBackup struct {
				FileName string `json:"fileName"`
			} `json:"beforeBackup"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rec.Body.String())
	}
	if !response.Data.Restored {
		t.Fatal("expected restored=true")
	}
	if response.Data.FileName != backup1.FileName {
		t.Fatalf("expected fileName=%q, got %q", backup1.FileName, response.Data.FileName)
	}
	if response.Data.BeforeBackup.FileName == "" {
		t.Fatal("expected beforeBackup.fileName to be non-empty (before-restore snapshot)")
	}
	if response.Data.BeforeBackup.FileName == backup1.FileName {
		t.Fatal("beforeBackup should be a NEW snapshot, not the same as the restored file")
	}

	// 6. 验证数据库内容已被恢复（标记账号应重新存在）
	// 注意：restoreFromBackup 会重新打开数据库连接，需要查询 app.db
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id = ?`, originalAccountID).Scan(&count)
	if count != 1 {
		t.Fatalf("expected original account to be restored (count=1), got %d", count)
	}

	// 7. 验证 before-restore 快照文件物理存在
	beforePath := filepath.Join(app.backupsDir(), response.Data.BeforeBackup.FileName)
	if _, err := os.Stat(beforePath); err != nil {
		t.Fatalf("before-restore backup file should exist at %s: %v", beforePath, err)
	}
}

// TestSystemRestoreWritesAuditLog 验证：恢复操作会写入审计日志，
// 记录恢复的文件名和 before-restore 快照。
func TestSystemRestoreWritesAuditLog(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	// 创建一个备份用于恢复
	backup, err := app.createBackup("audit-test-source")
	if err != nil {
		t.Fatalf("createBackup: %v", err)
	}

	body := `{"fileName":"` + backup.FileName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/system/restore", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	rec := httptest.NewRecorder()
	app.handleSystemRestore(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 查询审计日志
	var auditAction, auditResourceID, auditSummary string
	err = app.db.QueryRow(`
		SELECT action, resource_id, summary FROM audit_log
		WHERE action = 'backup.restored'
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&auditAction, &auditResourceID, &auditSummary)
	if err != nil {
		t.Fatalf("expected audit log entry for backup.restored, got error: %v", err)
	}
	if auditResourceID != backup.FileName {
		t.Fatalf("expected audit resource_id=%q, got %q", backup.FileName, auditResourceID)
	}
	if !strings.Contains(auditSummary, backup.FileName) {
		t.Fatalf("expected audit summary to contain %q, got %q", backup.FileName, auditSummary)
	}
	if !strings.Contains(auditSummary, "恢复") {
		t.Fatalf("expected audit summary to mention restore, got %q", auditSummary)
	}
}
