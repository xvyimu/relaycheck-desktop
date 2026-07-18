package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"relaycheck-desktop/internal/accounts"
)

func TestWritePublicErrorDoesNotExposeInternalCause(t *testing.T) {
	rec := httptest.NewRecorder()
	writePublicError(rec, http.StatusBadRequest, "导入失败，请检查输入。", errors.New(`open C:\\secret\\relaycheck.db: token=TOP_SECRET`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var response apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error != "导入失败，请检查输入。" || response.ErrorClass != "validation_error" {
		t.Fatalf("unexpected public response: %#v", response)
	}
	for _, forbidden := range []string{"C:\\secret", "relaycheck.db", "TOP_SECRET"} {
		if strings.Contains(response.Error, forbidden) {
			t.Fatalf("public error leaked %q: %q", forbidden, response.Error)
		}
	}
}

func TestSQLiteImportDoesNotExposeSourcePath(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/local-newapi/import-from-sqlite", strings.NewReader(`{"databasePath":"C:\\\\secret\\\\relaycheck.db"}`))
	rec := httptest.NewRecorder()
	app.handleImportFromSQLite(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response apiResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error != "SQLite 数据库路径不在允许的扫描目录内。" {
		t.Fatalf("unexpected public error: %q", response.Error)
	}
	for _, forbidden := range []string{"C:\\secret", "relaycheck.db", "SELECT", "token="} {
		if strings.Contains(response.Error, forbidden) {
			t.Fatalf("public error leaked %q: %q", forbidden, response.Error)
		}
	}
}

func TestWriteImportFailureUsesStableTypedContracts(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{name: "path", err: accounts.ErrSQLitePathRejected, wantStatus: http.StatusBadRequest, wantError: "SQLite 数据库路径不在允许的扫描目录内。"},
		{name: "format", err: accounts.ErrImportInvalidFormat, wantStatus: http.StatusBadRequest, wantError: "导入文件格式或结构无效。"},
		{name: "auth", err: accounts.ErrImportUpstreamAuth, wantStatus: http.StatusBadRequest, wantError: "上游认证失败，请检查访问令牌和权限。"},
		{name: "upstream", err: accounts.ErrImportUpstreamUnavailable, wantStatus: http.StatusBadRequest, wantError: "暂时无法读取上游数据，请稍后重试。"},
		{name: "storage", err: accounts.ErrImportStorage, wantStatus: http.StatusInternalServerError, wantError: "服务暂时不可用，请稍后重试。"},
		{name: "unknown", err: errors.New("C:\\secret\\db token=TOP_SECRET"), wantStatus: http.StatusInternalServerError, wantError: "服务暂时不可用，请稍后重试。"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeImportFailure(rec, tc.err, importFailureMessages{
				PathRejected:        "SQLite 数据库路径不在允许的扫描目录内。",
				InvalidFormat:       "导入文件格式或结构无效。",
				UpstreamAuth:        "上游认证失败，请检查访问令牌和权限。",
				UpstreamUnavailable: "暂时无法读取上游数据，请稍后重试。",
			})

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			var response apiResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != tc.wantError {
				t.Fatalf("error = %q, want %q", response.Error, tc.wantError)
			}
			if strings.Contains(response.Error, "TOP_SECRET") || strings.Contains(response.Error, "C:\\secret") {
				t.Fatalf("response leaked internal cause: %q", response.Error)
			}
		})
	}
}
