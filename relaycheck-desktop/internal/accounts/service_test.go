package accounts

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// stubInfra implements accounts.Infra for testing.
type stubInfra struct {
	db         *sql.DB
	encryptFn  func(string) (string, error)
	decryptFn  func(string) (string, error)
	detectFn   func(context.Context, string) (Detection, error)
	ensureFn   func(context.Context, string, string, string, string, *Detection) (string, bool, error)
	doHTTPFn   func(*http.Request) (*http.Response, error)
	notifyFn   func(kind, level, title, content, relatedType, relatedID string)
	auditFn    func(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{})
	nowFn      func() string
	newIDFn    func() string
}

var _ Infra = (*stubInfra)(nil)

func (s *stubInfra) DB() *sql.DB                               { return s.db }
func (s *stubInfra) DoHTTP(req *http.Request) (*http.Response, error) {
	if s.doHTTPFn != nil {
		return s.doHTTPFn(req)
	}
	return nil, nil
}
func (s *stubInfra) EncryptText(plaintext string) (string, error) {
	if s.encryptFn != nil {
		return s.encryptFn(plaintext)
	}
	return "enc:" + plaintext, nil
}
func (s *stubInfra) DecryptText(ciphertext string) (string, error) {
	if s.decryptFn != nil {
		return s.decryptFn(ciphertext)
	}
	return strings.TrimPrefix(ciphertext, "enc:"), nil
}
func (s *stubInfra) DetectUpstreamForImport(ctx context.Context, raw string) (Detection, error) {
	if s.detectFn != nil {
		return s.detectFn(ctx, raw)
	}
	return Detection{BaseURL: raw, Kind: "newapi"}, nil
}
func (s *stubInfra) EnsureChannelSiteForImport(ctx context.Context, channelID, name, rawBaseURL, kind string, detection *Detection) (string, bool, error) {
	if s.ensureFn != nil {
		return s.ensureFn(ctx, channelID, name, rawBaseURL, kind, detection)
	}
	return "site-" + channelID, true, nil
}
func (s *stubInfra) Notify(kind, level, title, content, relatedType, relatedID string) {
	if s.notifyFn != nil {
		s.notifyFn(kind, level, title, content, relatedType, relatedID)
	}
}
func (s *stubInfra) Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{}) {
	if s.auditFn != nil {
		s.auditFn(action, level, userID, entityType, entityID, detail, metadata)
	}
}
func (s *stubInfra) Now() string {
	if s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now().UTC().Format(time.RFC3339)
}
func (s *stubInfra) NewID() string {
	if s.newIDFn != nil {
		return s.newIDFn()
	}
	return "tid-" + strings.Repeat("a", 24)
}

// setupAccountsTestDB creates an in-memory SQLite database with the tables
// needed by accounts.Service methods.
func setupAccountsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schema := `
CREATE TABLE IF NOT EXISTS local_newapi_instances (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	base_url TEXT NOT NULL UNIQUE,
	detected_from TEXT,
	status TEXT NOT NULL DEFAULT 'unknown',
	version TEXT,
	database_path TEXT,
	last_scanned_at TEXT,
	sync_access_token_encrypted TEXT,
	sync_access_token_masked TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS upstream_sites (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	base_url TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'unknown'
);
CREATE TABLE IF NOT EXISTS imported_channels (
	id TEXT PRIMARY KEY,
	local_instance_id TEXT,
	source_channel_id TEXT NOT NULL,
	name TEXT NOT NULL,
	base_url TEXT,
	status TEXT,
	upstream_kind TEXT NOT NULL DEFAULT 'unknown',
	raw_json TEXT NOT NULL,
	source_sync_status TEXT NOT NULL DEFAULT 'active',
	source_missing_at TEXT,
	model_count INTEGER NOT NULL DEFAULT 0,
	sample_models_json TEXT,
	models_status TEXT,
	models_source TEXT,
	models_last_synced_at TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(local_instance_id, source_channel_id)
);
CREATE TABLE IF NOT EXISTS channel_accounts (
	id TEXT PRIMARY KEY,
	upstream_site_id TEXT NOT NULL,
	display_name TEXT NOT NULL,
	username TEXT,
	email TEXT,
	auth_type TEXT NOT NULL,
	api_key_encrypted TEXT,
	password_encrypted TEXT,
	login_status TEXT NOT NULL DEFAULT 'unknown',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`
	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestListLocalNewAPIInstances(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	// Empty table → empty slice.
	items, err := svc.ListLocalNewAPIInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}

	// Insert an instance.
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"inst-1", "Test Instance", "https://test.example", "active", now, now)
	if err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	items, err = svc.ListLocalNewAPIInstances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "Test Instance" {
		t.Errorf("Name = %q, want %q", items[0].Name, "Test Instance")
	}
	if items[0].BaseURL != "https://test.example" {
		t.Errorf("BaseURL = %q, want %q", items[0].BaseURL, "https://test.example")
	}
	if items[0].ChannelCount != 0 {
		t.Errorf("ChannelCount = %d, want 0", items[0].ChannelCount)
	}
}

func TestGetLocalNewAPIInstance(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	// Non-existent instance → ErrNoRows.
	_, err := svc.GetLocalNewAPIInstance(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got: %v", err)
	}

	// Insert and retrieve.
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at, sync_access_token_encrypted, sync_access_token_masked)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"inst-1", "Test Instance", "https://test.example", "active", now, now, "encrypted-token", "****oken")
	if err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	inst, err := svc.GetLocalNewAPIInstance(context.Background(), "inst-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Name != "Test Instance" {
		t.Errorf("Name = %q, want %q", inst.Name, "Test Instance")
	}
	if !inst.HasSyncToken {
		t.Error("expected HasSyncToken=true for masked token")
	}
}

func TestUpdateLocalNewAPISyncToken(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"inst-1", "Test Instance", "https://test.example", "active", now, now)
	if err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	// Save token.
	encryptFn := func(plain string) (string, error) { return "enc:" + plain, nil }
	infra.encryptFn = encryptFn

	err = svc.UpdateLocalNewAPISyncToken(context.Background(), "inst-1", "my-token", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var encrypted, masked string
	err = db.QueryRow(`SELECT sync_access_token_encrypted, sync_access_token_masked FROM local_newapi_instances WHERE id=?`, "inst-1").
		Scan(&encrypted, &masked)
	if err != nil {
		t.Fatalf("query token: %v", err)
	}
	if encrypted != "enc:my-token" {
		t.Errorf("encrypted = %q, want %q", encrypted, "enc:my-token")
	}
	if masked == "" {
		t.Error("expected masked token to be non-empty")
	}

	// Clear token.
	err = svc.UpdateLocalNewAPISyncToken(context.Background(), "inst-1", "", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = db.QueryRow(`SELECT sync_access_token_encrypted FROM local_newapi_instances WHERE id=?`, "inst-1").
		Scan(&encrypted)
	if err != nil {
		t.Fatalf("query token: %v", err)
	}
	if encrypted != "" {
		t.Errorf("expected cleared token, got %q", encrypted)
	}

	// No-op: !save and !clear.
	err = svc.UpdateLocalNewAPISyncToken(context.Background(), "inst-1", "new-token", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncLocalNewAPIInstanceData_UsesSQLite(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, database_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"inst-1", "SQLite Instance", "https://test.example", "active", "/path/to/test.db", now, now)
	if err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	_, err = svc.SyncLocalNewAPIInstanceData(context.Background(), "inst-1", SyncRunInput{}, false)
	// ImportChannelsFromSQLite opens the actual file at /path/to/test.db,
	// which doesn't exist — expect an error path, not a panic.
	if err == nil {
		t.Fatal("expected error for non-existent SQLite file")
	}
}

func TestSyncLocalNewAPIInstanceData_ErrNoRows(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	_, err := svc.SyncLocalNewAPIInstanceData(context.Background(), "nonexistent", SyncRunInput{}, false)
	if err == nil {
		t.Fatal("expected error for non-existent instance")
	}
	if !strings.Contains(err.Error(), "不存在") {
		t.Errorf("expected '不存在' in error, got: %v", err)
	}
}

func TestBaseURLForAutoDetectedDB(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)

	cases := []struct {
		dbPath string
		want   string
	}{
		{"/data/newapi/db.sqlite", "http://127.0.0.1:3000"},
		{"/data/oneapi/one-api.db", "http://127.0.0.1:3000"},
		{"/data/something-else/data.db", ""},
	}
	for _, tc := range cases {
		got := svc.BaseURLForAutoDetectedDB(context.Background(), tc.dbPath)
		if got != tc.want {
			t.Errorf("BaseURLForAutoDetectedDB(%q) = %q, want %q", tc.dbPath, got, tc.want)
		}
	}
}

func TestMaskSecretByteLength(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"", ""},
		{"abc", "***"},
		{"abcd", "****"},
		{"sk-1234567890", "*********7890"},
	}
	for _, tc := range cases {
		got := maskSecret(tc.value)
		if got != tc.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}
