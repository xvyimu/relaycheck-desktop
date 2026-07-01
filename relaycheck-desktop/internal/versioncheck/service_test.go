package versioncheck

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	_ "modernc.org/sqlite"
)

// stubInfra implements the Infra interface for tests.
type stubInfra struct {
	db              *sql.DB
	httpClient      *http.Client
	productVersion  string
	validateURLFn   func(ctx context.Context, raw string) (*url.URL, error)
}

func (s *stubInfra) DB() *sql.DB                                     { return s.db }
func (s *stubInfra) HTTPClient() *http.Client                        { return s.httpClient }
func (s *stubInfra) ProductVersion() string                           { return s.productVersion }
func (s *stubInfra) ValidateOutboundURLStrict(ctx context.Context, raw string) (*url.URL, error) {
	if s.validateURLFn != nil {
		return s.validateURLFn(ctx, raw)
	}
	return url.Parse(raw)
}

// setupTestDB creates an in-memory SQLite database with a system_settings
// table and inserts the app.version_check_url setting with the given value.
func setupTestDB(t *testing.T, versionCheckURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS system_settings (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		value_json TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create table: %v", err)
	}
	if versionCheckURL != "" {
		encoded, _ := json.Marshal(versionCheckURL)
		_, err = db.Exec(`INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES ('vc', 'app.version_check_url', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, string(encoded))
		if err != nil {
			db.Close()
			t.Fatalf("insert setting: %v", err)
		}
	}
	return db
}

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		name           string
		productVersion string
		settingURL     string
		manifestResp   interface{} // JSON response from the server, or int for HTTP status
		manifestStatus int
		wantAvailable  bool
		wantError      string // substring to match in Error field; empty means no error expected
		useStrictBlock bool // if true, ValidateOutboundURLStrict returns error
	}{
		{
			name:           "newer_version_available",
			productVersion: "1.0.0",
			settingURL:     "PLACEHOLDER", // replaced by httptest server URL
			manifestResp: versionManifest{
				Version:      "1.1.0",
				ReleaseURL:   "https://example.com/release",
				ReleaseNotes: "Bug fixes and improvements",
			},
			manifestStatus: http.StatusOK,
			wantAvailable:  true,
		},
		{
			name:           "same_version",
			productVersion: "1.0.0",
			settingURL:     "PLACEHOLDER",
			manifestResp: versionManifest{
				Version:      "1.0.0",
				ReleaseURL:   "https://example.com/release",
				ReleaseNotes: "Current release",
			},
			manifestStatus: http.StatusOK,
			wantAvailable:  false,
		},
		{
			name:           "older_remote_version",
			productVersion: "2.0.0",
			settingURL:     "PLACEHOLDER",
			manifestResp: versionManifest{
				Version:      "1.0.0",
				ReleaseURL:   "https://example.com/release",
				ReleaseNotes: "Old release",
			},
			manifestStatus: http.StatusOK,
			wantAvailable:  false,
		},
		{
			name:           "server_error_500",
			productVersion: "1.0.0",
			settingURL:     "PLACEHOLDER",
			manifestResp:   nil,
			manifestStatus: http.StatusInternalServerError,
			wantAvailable:  false,
			wantError:      "HTTP 500",
		},
		{
			name:           "invalid_json_response",
			productVersion: "1.0.0",
			settingURL:     "PLACEHOLDER",
			manifestResp:   "not-valid-json{{{",
			manifestStatus: http.StatusOK,
			wantAvailable:  false,
			wantError:      "解析版本清单失败",
		},
		{
			name:           "missing_setting_url",
			productVersion: "1.0.0",
			settingURL:     "",
			manifestResp:   nil,
			manifestStatus: http.StatusOK,
			wantAvailable:  false,
			wantError:      "未配置版本检查 URL",
		},
		{
			name:           "url_validation_fails",
			productVersion: "1.0.0",
			settingURL:     "PLACEHOLDER",
			manifestResp:   nil,
			manifestStatus: http.StatusOK,
			wantAvailable:  false,
			wantError:      "版本检查 URL 校验失败",
			useStrictBlock: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up a mock HTTP server that returns the configured manifest response.
			var serverURL string
			var httpClient *http.Client
			if tc.settingURL == "PLACEHOLDER" {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.manifestStatus != http.StatusOK {
						w.WriteHeader(tc.manifestStatus)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					switch v := tc.manifestResp.(type) {
					case string:
						// Return raw string (for invalid JSON test)
						w.Write([]byte(v))
					default:
						json.NewEncoder(w).Encode(v)
					}
				}))
				defer srv.Close()
				serverURL = srv.URL
				httpClient = srv.Client()
			} else {
				httpClient = &http.Client{}
			}

			// Replace PLACEHOLDER with server URL.
			settingURL := tc.settingURL
			if settingURL == "PLACEHOLDER" {
				settingURL = serverURL
			}

			// Set up the test database.
			db := setupTestDB(t, settingURL)
			defer db.Close()

			// Build the stub infra.
			infra := &stubInfra{
				db:             db,
				httpClient:     httpClient,
				productVersion: tc.productVersion,
			}
			if tc.useStrictBlock {
				infra.validateURLFn = func(ctx context.Context, raw string) (*url.URL, error) {
					return nil, &url.Error{Op: "Get", URL: raw, Err: http.ErrUseLastResponse}
				}
			}

			svc := NewService(infra)
			result := svc.CheckVersion(context.Background())

			if result.CurrentVersion != tc.productVersion {
				t.Errorf("CurrentVersion = %q, want %q", result.CurrentVersion, tc.productVersion)
			}
			if result.UpdateAvailable != tc.wantAvailable {
				t.Errorf("UpdateAvailable = %v, want %v", result.UpdateAvailable, tc.wantAvailable)
			}
			if tc.wantError != "" {
				if result.Error == "" {
					t.Errorf("expected error containing %q, got empty error", tc.wantError)
				}
			} else if result.Error != "" {
				t.Errorf("unexpected error: %q", result.Error)
			}
		})
	}
}

func TestCheckVersion_ServerUnreachable(t *testing.T) {
	// Use a port that nobody listens on so the HTTP request fails.
	db := setupTestDB(t, "http://127.0.0.1:1/manifest.json")
	defer db.Close()

	infra := &stubInfra{
		db:             db,
		httpClient:     &http.Client{},
		productVersion: "1.0.0",
		// Override ValidateOutboundURLStrict to accept localhost for this test,
		// since the strict policy blocks local addresses.
		validateURLFn: func(_ context.Context, raw string) (*url.URL, error) {
			return url.Parse(raw)
		},
	}

	svc := NewService(infra)
	result := svc.CheckVersion(context.Background())

	if result.Error == "" {
		t.Error("expected error for unreachable server, got empty error")
	}
	if result.UpdateAvailable {
		t.Error("UpdateAvailable should be false when server is unreachable")
	}
}

func TestCheckVersion_SetsCheckedAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(versionManifest{
			Version:      "1.0.0",
			ReleaseURL:   "https://example.com",
			ReleaseNotes: "test",
		})
	}))
	defer srv.Close()

	db := setupTestDB(t, srv.URL)
	defer db.Close()

	infra := &stubInfra{
		db:             db,
		httpClient:     srv.Client(),
		productVersion: "1.0.0",
	}

	svc := NewService(infra)
	result := svc.CheckVersion(context.Background())

	if result.CheckedAt == "" {
		t.Error("CheckedAt should not be empty")
	}
}

func TestCheckVersion_SetsManifestFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(versionManifest{
			Version:      "2.5.0",
			ReleaseURL:   "https://example.com/v2.5.0",
			ReleaseNotes: "Major update",
		})
	}))
	defer srv.Close()

	db := setupTestDB(t, srv.URL)
	defer db.Close()

	infra := &stubInfra{
		db:             db,
		httpClient:     srv.Client(),
		productVersion: "1.0.0",
	}

	svc := NewService(infra)
	result := svc.CheckVersion(context.Background())

	if result.LatestVersion != "2.5.0" {
		t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "2.5.0")
	}
	if result.ReleaseURL != "https://example.com/v2.5.0" {
		t.Errorf("ReleaseURL = %q, want %q", result.ReleaseURL, "https://example.com/v2.5.0")
	}
	if result.ReleaseNotes != "Major update" {
		t.Errorf("ReleaseNotes = %q, want %q", result.ReleaseNotes, "Major update")
	}
}

func TestCheckVersion_RequestHeaders(t *testing.T) {
	var gotUA string
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		json.NewEncoder(w).Encode(versionManifest{Version: "1.0.0"})
	}))
	defer srv.Close()

	db := setupTestDB(t, srv.URL)
	defer db.Close()

	infra := &stubInfra{
		db:             db,
		httpClient:     srv.Client(),
		productVersion: "3.2.1",
	}

	svc := NewService(infra)
	svc.CheckVersion(context.Background())

	if gotUA != "RelayCheck-Desktop/3.2.1" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "RelayCheck-Desktop/3.2.1")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

func TestGetSettingString(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS system_settings (
		id TEXT PRIMARY KEY,
		key TEXT NOT NULL UNIQUE,
		value_json TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	svc := NewService(&stubInfra{db: db})

	t.Run("missing_key_returns_empty", func(t *testing.T) {
		got := svc.getSettingString(context.Background(), "nonexistent")
		if got != "" {
			t.Errorf("expected empty string for missing key, got %q", got)
		}
	})

	t.Run("json_encoded_value", func(t *testing.T) {
		encoded, _ := json.Marshal("https://example.com")
		_, err := db.Exec(`INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES ('t1', 'test.key', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, string(encoded))
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		got := svc.getSettingString(context.Background(), "test.key")
		if got != "https://example.com" {
			t.Errorf("got %q, want %q", got, "https://example.com")
		}
	})

	t.Run("raw_value", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES ('t2', 'test.raw', 'raw-value', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		got := svc.getSettingString(context.Background(), "test.raw")
		if got != "raw-value" {
			t.Errorf("got %q, want %q", got, "raw-value")
		}
	})
}
