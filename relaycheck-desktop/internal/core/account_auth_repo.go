package core

import (
	"context"
	"database/sql"
	"strings"
)

// AccountAuthRepository loads accountAuthContext records from the database.
// It is extracted from *App.loadAccountAuth / *App.loadAccountAuths so that
// the SQL and decryption logic can be reused without going through the App
// god object. The *App methods are retained as thin forwarders so existing
// callers (a.loadAccountAuth / a.loadAccountAuths) are unaffected.
type AccountAuthRepository struct {
	db     *sql.DB
	crypto *CryptoService
}

// NewAccountAuthRepository constructs an AccountAuthRepository backed by the
// given database handle and crypto service.
func NewAccountAuthRepository(db *sql.DB, crypto *CryptoService) *AccountAuthRepository {
	return &AccountAuthRepository{db: db, crypto: crypto}
}

// Load fetches a single accountAuthContext by account ID. If the account is
// not found, the error reports "账号不存在。". Decryption failures are
// ignored (matching the original loadAccountAuth behaviour, which discarded
// the decrypt error with "_").
func (r *AccountAuthRepository) Load(ctx context.Context, id string) (*accountAuthContext, error) {
	var auth accountAuthContext
	var email, username, cookieEncrypted, accessEncrypted, apiKeyEncrypted, passwordEncrypted, loginURL, checkinConfigJSON string
	var supportsCheckin, supportsBalance int
	var siteKind sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, COALESCE(s.kind,''), COALESCE(s.channel_id,''), s.base_url,
		       COALESCE(s.login_url,''), COALESCE(a.user_agent,''), COALESCE(a.email,''), COALESCE(a.username,''),
		       COALESCE(a.password_encrypted,''), COALESCE(a.cookie_encrypted,''),
		       COALESCE(a.access_token_encrypted,''), COALESCE(a.api_key_encrypted,''),
		       COALESCE(a.auth_user_id,''), s.supports_checkin, s.supports_balance,
		       COALESCE(s.checkin_config_json,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id = ?
	`, id).Scan(&auth.AccountID, &auth.AccountName, &auth.UpstreamSiteID, &auth.UpstreamSite, &siteKind, &auth.ChannelID, &auth.BaseURL, &loginURL, &auth.UserAgent, &email, &username, &passwordEncrypted, &cookieEncrypted, &accessEncrypted, &apiKeyEncrypted, &auth.AuthUserID, &supportsCheckin, &supportsBalance, &checkinConfigJSON)
	if err == sql.ErrNoRows {
		return nil, errorsText("账号不存在。")
	}
	if err != nil {
		return nil, err
	}
	auth.LoginName = firstNonEmpty(email, username)
	auth.SiteKind = siteKind.String
	auth.LoginPath = pathFromMaybeURL(loginURL)
	auth.Password, _ = r.crypto.Decrypt(passwordEncrypted)
	auth.Cookie, _ = r.crypto.Decrypt(cookieEncrypted)
	auth.AccessToken, _ = r.crypto.Decrypt(accessEncrypted)
	auth.APIKey, _ = r.crypto.Decrypt(apiKeyEncrypted)
	auth.SupportsCheckin = supportsCheckin == 1
	auth.SupportsBalance = supportsBalance == 1
	auth.CheckinRules = parseCheckinRules(checkinConfigJSON)
	if len(auth.CheckinRules) > 0 {
		auth.SupportsCheckin = true
	}
	return &auth, nil
}

// LoadBatch batch-loads accountAuthContext for multiple account IDs in a
// single query, eliminating N+1 lookups in bulk operations. Returns a map
// keyed by account ID. If a particular ID is not found, it is simply absent
// from the map; callers should fall back to Load for missing entries.
func (r *AccountAuthRepository) LoadBatch(ctx context.Context, ids []string) (map[string]accountAuthContext, error) {
	if len(ids) == 0 {
		return map[string]accountAuthContext{}, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, COALESCE(s.kind,''), COALESCE(s.channel_id,''), s.base_url,
		       COALESCE(s.login_url,''), COALESCE(a.user_agent,''), COALESCE(a.email,''), COALESCE(a.username,''),
		       COALESCE(a.password_encrypted,''), COALESCE(a.cookie_encrypted,''),
		       COALESCE(a.access_token_encrypted,''), COALESCE(a.api_key_encrypted,''),
		       COALESCE(a.auth_user_id,''), s.supports_checkin, s.supports_balance,
		       COALESCE(s.checkin_config_json,'')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	auths := make(map[string]accountAuthContext, len(ids))
	for rows.Next() {
		var auth accountAuthContext
		var email, username, cookieEncrypted, accessEncrypted, apiKeyEncrypted, passwordEncrypted, loginURL, checkinConfigJSON string
		var supportsCheckin, supportsBalance int
		var siteKind sql.NullString
		if err := rows.Scan(&auth.AccountID, &auth.AccountName, &auth.UpstreamSiteID, &auth.UpstreamSite, &siteKind, &auth.ChannelID, &auth.BaseURL, &loginURL, &auth.UserAgent, &email, &username, &passwordEncrypted, &cookieEncrypted, &accessEncrypted, &apiKeyEncrypted, &auth.AuthUserID, &supportsCheckin, &supportsBalance, &checkinConfigJSON); err != nil {
			return nil, err
		}
		auth.LoginName = firstNonEmpty(email, username)
		auth.SiteKind = siteKind.String
		auth.LoginPath = pathFromMaybeURL(loginURL)
		auth.Password, _ = r.crypto.Decrypt(passwordEncrypted)
		auth.Cookie, _ = r.crypto.Decrypt(cookieEncrypted)
		auth.AccessToken, _ = r.crypto.Decrypt(accessEncrypted)
		auth.APIKey, _ = r.crypto.Decrypt(apiKeyEncrypted)
		auth.SupportsCheckin = supportsCheckin == 1
		auth.SupportsBalance = supportsBalance == 1
		auth.CheckinRules = parseCheckinRules(checkinConfigJSON)
		if len(auth.CheckinRules) > 0 {
			auth.SupportsCheckin = true
		}
		auths[auth.AccountID] = auth
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return auths, nil
}
