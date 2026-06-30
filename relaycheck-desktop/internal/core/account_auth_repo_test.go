package core

import (
	"context"
	"strings"
	"testing"
)

// insertTestSite inserts a minimal upstream_sites row and returns its id.
// Required NOT NULL columns (id, name, base_url, created_at, updated_at) are
// populated; optional columns default via the schema.
func insertTestSite(t *testing.T, app *App, id, name, baseURL, kind string) {
	t.Helper()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'unknown', ?, ?)
	`, id, name, baseURL, kind, now(), now()); err != nil {
		t.Fatalf("insert upstream_sites: %v", err)
	}
}

// insertTestAccount inserts a channel_accounts row tied to the given site id.
// Only the NOT NULL columns are populated; callers pass extra columns via the
// two parallel slices (cols/vals) for encrypted credentials, email, etc.
func insertTestAccount(t *testing.T, app *App, id, siteID, displayName string, extraCols []string, extraVals []interface{}) {
	t.Helper()
	if len(extraCols) != len(extraVals) {
		t.Fatalf("extraCols/extraVals length mismatch: %d vs %d", len(extraCols), len(extraVals))
	}
	cols := []string{"id", "upstream_site_id", "display_name", "auth_type", "login_status", "created_at", "updated_at"}
	vals := []interface{}{id, siteID, displayName, "email_password", "unknown", now(), now()}
	cols = append(cols, extraCols...)
	vals = append(vals, extraVals...)
	placeholder := "?, ?, ?, ?, ?, ?, ?"
	for range extraCols {
		placeholder += ", ?"
	}
	q := "INSERT INTO channel_accounts (" + strings.Join(cols, ", ") + ") VALUES (" + placeholder + ")"
	if _, err := app.db.Exec(q, vals...); err != nil {
		t.Fatalf("insert channel_accounts: %v", err)
	}
}

func TestAccountAuthRepository_LoadMissingAccount(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	_, err := app.accountAuth.Load(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing account, got nil")
	}
	if !strings.Contains(err.Error(), "账号不存在") {
		t.Fatalf("expected '账号不存在' error, got %q", err.Error())
	}
	// Note: errorsText() returns a fresh fmt.Errorf each call, so callers
	// cannot use errors.Is against a sentinel to detect missing accounts —
	// they must match on the message text. This is the legacy behaviour
	// preserved by the repository extraction.
}

func TestAccountAuthRepository_LoadValidAccount(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	insertTestSite(t, app, siteID, "Example Relay", "https://relay.example", "newapi")
	insertTestAccount(t, app, accountID, siteID, "Display Name", []string{
		"email", "username", "user_agent", "auth_user_id",
	}, []interface{}{"user@example.com", "uname", "UA/1.0", "auth-uid-1"})

	auth, err := app.accountAuth.Load(context.Background(), accountID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if auth.AccountID != accountID {
		t.Errorf("AccountID = %q, want %q", auth.AccountID, accountID)
	}
	if auth.AccountName != "Display Name" {
		t.Errorf("AccountName = %q, want %q", auth.AccountName, "Display Name")
	}
	if auth.UpstreamSiteID != siteID {
		t.Errorf("UpstreamSiteID = %q, want %q", auth.UpstreamSiteID, siteID)
	}
	if auth.UpstreamSite != "Example Relay" {
		t.Errorf("UpstreamSite = %q, want %q", auth.UpstreamSite, "Example Relay")
	}
	if auth.SiteKind != "newapi" {
		t.Errorf("SiteKind = %q, want %q", auth.SiteKind, "newapi")
	}
	if auth.BaseURL != "https://relay.example" {
		t.Errorf("BaseURL = %q, want %q", auth.BaseURL, "https://relay.example")
	}
	if auth.LoginName != "user@example.com" {
		t.Errorf("LoginName = %q, want %q (email preferred over username)", auth.LoginName, "user@example.com")
	}
	if auth.UserAgent != "UA/1.0" {
		t.Errorf("UserAgent = %q, want %q", auth.UserAgent, "UA/1.0")
	}
	if auth.AuthUserID != "auth-uid-1" {
		t.Errorf("AuthUserID = %q, want %q", auth.AuthUserID, "auth-uid-1")
	}
	if auth.SupportsCheckin {
		t.Errorf("SupportsCheckin = true, want false (no rules + flag 0)")
	}
	if auth.SupportsBalance {
		t.Errorf("SupportsBalance = true, want false")
	}
	if auth.CheckinRules != nil {
		t.Errorf("CheckinRules = %#v, want nil", auth.CheckinRules)
	}
}

func TestAccountAuthRepository_LoadDecryptsCredentials(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	insertTestSite(t, app, siteID, "Encrypted Site", "https://relay.example", "newapi")

	passwordEnc, err := app.encryptText("s3cret-pw")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	cookieEnc, err := app.encryptText("session=abc123")
	if err != nil {
		t.Fatalf("encrypt cookie: %v", err)
	}
	accessEnc, err := app.encryptText("access-token-xyz")
	if err != nil {
		t.Fatalf("encrypt access_token: %v", err)
	}
	apiKeyEnc, err := app.encryptText("sk-api-key")
	if err != nil {
		t.Fatalf("encrypt api_key: %v", err)
	}
	insertTestAccount(t, app, accountID, siteID, "Encrypted Account", []string{
		"password_encrypted", "cookie_encrypted", "access_token_encrypted", "api_key_encrypted",
	}, []interface{}{passwordEnc, cookieEnc, accessEnc, apiKeyEnc})

	auth, err := app.accountAuth.Load(context.Background(), accountID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if auth.Password != "s3cret-pw" {
		t.Errorf("Password = %q, want decrypted %q", auth.Password, "s3cret-pw")
	}
	if auth.Cookie != "session=abc123" {
		t.Errorf("Cookie = %q, want decrypted %q", auth.Cookie, "session=abc123")
	}
	if auth.AccessToken != "access-token-xyz" {
		t.Errorf("AccessToken = %q, want decrypted %q", auth.AccessToken, "access-token-xyz")
	}
	if auth.APIKey != "sk-api-key" {
		t.Errorf("APIKey = %q, want decrypted %q", auth.APIKey, "sk-api-key")
	}
}

func TestAccountAuthRepository_LoadBatchEmpty(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	got, err := app.accountAuth.LoadBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("LoadBatch(nil) returned error: %v", err)
	}
	if got == nil {
		t.Fatal("LoadBatch(nil) returned nil map, want non-nil empty map")
	}
	if len(got) != 0 {
		t.Fatalf("LoadBatch(nil) returned %d entries, want 0", len(got))
	}

	got2, err := app.accountAuth.LoadBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("LoadBatch([]) returned error: %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("LoadBatch([]) returned %d entries, want 0", len(got2))
	}
}

func TestAccountAuthRepository_LoadBatchMixed(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	insertTestSite(t, app, siteID, "Batch Site", "https://relay.example", "newapi")

	id1 := newID()
	id2 := newID()
	missingID := newID()
	insertTestAccount(t, app, id1, siteID, "Account 1", nil, nil)
	insertTestAccount(t, app, id2, siteID, "Account 2", nil, nil)

	got, err := app.accountAuth.LoadBatch(context.Background(), []string{id1, id2, missingID})
	if err != nil {
		t.Fatalf("LoadBatch returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadBatch returned %d entries, want 2 (missing must be silently absent)", len(got))
	}
	if _, ok := got[id1]; !ok {
		t.Errorf("expected map to contain id1=%q", id1)
	}
	if _, ok := got[id2]; !ok {
		t.Errorf("expected map to contain id2=%q", id2)
	}
	if _, ok := got[missingID]; ok {
		t.Errorf("expected missing id=%q to be absent from map", missingID)
	}
}

func TestAccountAuthRepository_LoadBatchPreservesFields(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	insertTestSite(t, app, siteID, "Fields Site", "https://relay.example", "newapi")

	accountID := newID()
	passwordEnc, err := app.encryptText("batch-pw")
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	cookieEnc, err := app.encryptText("batch-cookie")
	if err != nil {
		t.Fatalf("encrypt cookie: %v", err)
	}
	insertTestAccount(t, app, accountID, siteID, "Batch Fields Account", []string{
		"email", "username", "user_agent", "password_encrypted", "cookie_encrypted",
	}, []interface{}{"batch@example.com", "batchuser", "BatchUA/2.0", passwordEnc, cookieEnc})

	// Sanity: ensure the row exists in batch form.
	batch, err := app.accountAuth.LoadBatch(context.Background(), []string{accountID})
	if err != nil {
		t.Fatalf("LoadBatch returned error: %v", err)
	}
	batchAuth, ok := batch[accountID]
	if !ok {
		t.Fatalf("LoadBatch did not return entry for %q", accountID)
	}

	single, err := app.accountAuth.Load(context.Background(), accountID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	// Compare every field of accountAuthContext. Batch returns a value (not a
	// pointer) so we dereference single for the comparison.
	if batchAuth.AccountID != single.AccountID {
		t.Errorf("AccountID mismatch: batch=%q single=%q", batchAuth.AccountID, single.AccountID)
	}
	if batchAuth.AccountName != single.AccountName {
		t.Errorf("AccountName mismatch: batch=%q single=%q", batchAuth.AccountName, single.AccountName)
	}
	if batchAuth.UpstreamSiteID != single.UpstreamSiteID {
		t.Errorf("UpstreamSiteID mismatch: batch=%q single=%q", batchAuth.UpstreamSiteID, single.UpstreamSiteID)
	}
	if batchAuth.UpstreamSite != single.UpstreamSite {
		t.Errorf("UpstreamSite mismatch: batch=%q single=%q", batchAuth.UpstreamSite, single.UpstreamSite)
	}
	if batchAuth.SiteKind != single.SiteKind {
		t.Errorf("SiteKind mismatch: batch=%q single=%q", batchAuth.SiteKind, single.SiteKind)
	}
	if batchAuth.BaseURL != single.BaseURL {
		t.Errorf("BaseURL mismatch: batch=%q single=%q", batchAuth.BaseURL, single.BaseURL)
	}
	if batchAuth.LoginName != single.LoginName {
		t.Errorf("LoginName mismatch: batch=%q single=%q", batchAuth.LoginName, single.LoginName)
	}
	if batchAuth.UserAgent != single.UserAgent {
		t.Errorf("UserAgent mismatch: batch=%q single=%q", batchAuth.UserAgent, single.UserAgent)
	}
	if batchAuth.Password != single.Password {
		t.Errorf("Password mismatch: batch=%q single=%q", batchAuth.Password, single.Password)
	}
	if batchAuth.Cookie != single.Cookie {
		t.Errorf("Cookie mismatch: batch=%q single=%q", batchAuth.Cookie, single.Cookie)
	}
	if batchAuth.AccessToken != single.AccessToken {
		t.Errorf("AccessToken mismatch: batch=%q single=%q", batchAuth.AccessToken, single.AccessToken)
	}
	if batchAuth.APIKey != single.APIKey {
		t.Errorf("APIKey mismatch: batch=%q single=%q", batchAuth.APIKey, single.APIKey)
	}
	if batchAuth.AuthUserID != single.AuthUserID {
		t.Errorf("AuthUserID mismatch: batch=%q single=%q", batchAuth.AuthUserID, single.AuthUserID)
	}
	if batchAuth.SupportsCheckin != single.SupportsCheckin {
		t.Errorf("SupportsCheckin mismatch: batch=%v single=%v", batchAuth.SupportsCheckin, single.SupportsCheckin)
	}
	if batchAuth.SupportsBalance != single.SupportsBalance {
		t.Errorf("SupportsBalance mismatch: batch=%v single=%v", batchAuth.SupportsBalance, single.SupportsBalance)
	}
	if len(batchAuth.CheckinRules) != len(single.CheckinRules) {
		t.Errorf("CheckinRules length mismatch: batch=%d single=%d", len(batchAuth.CheckinRules), len(single.CheckinRules))
	}
}
