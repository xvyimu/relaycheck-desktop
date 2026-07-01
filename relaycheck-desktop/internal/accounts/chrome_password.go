package accounts

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
)

// PreviewChromePasswordImport parses a Chrome password CSV and matches each
// row against existing upstream_sites, returning a preview without writing.
func (s *Service) PreviewChromePasswordImport(ctx context.Context, csvContent string) (map[string]interface{}, error) {
	rows, err := parseChromePasswordCSV(csvContent)
	if err != nil {
		return nil, err
	}
	sites, err := s.loadPasswordSites(ctx)
	if err != nil {
		return nil, err
	}
	matches, err := s.matchChromePasswordRows(ctx, rows, sites)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"totalRows":       len(rows),
		"matchedRows":     len(matches),
		"uniqueSiteCount": countUniqueMatchedSites(matches),
		"matches":         matches,
	}, nil
}

// ImportChromePasswords parses a Chrome password CSV, matches rows against
// existing sites, and inserts new channel_accounts rows for non-existing
// matches.
func (s *Service) ImportChromePasswords(ctx context.Context, csvContent string) (map[string]interface{}, error) {
	rows, err := parseChromePasswordCSV(csvContent)
	if err != nil {
		return nil, err
	}
	sites, err := s.loadPasswordSites(ctx)
	if err != nil {
		return nil, err
	}
	matches, err := s.matchChromePasswordRows(ctx, rows, sites)
	if err != nil {
		return nil, err
	}

	imported := 0
	skippedExisting := 0
	seenAccounts := map[string]bool{}
	for _, match := range matches {
		accountKey := match.SiteID + "|" + strings.ToLower(strings.TrimSpace(match.Username))
		if seenAccounts[accountKey] {
			skippedExisting++
			continue
		}
		seenAccounts[accountKey] = true
		if match.ExistingAccount {
			skippedExisting++
			continue
		}
		row := findChromeRow(rows, match.URL, match.Username)
		if row.Password == "" {
			continue
		}
		encryptedPassword, err := s.infra.EncryptText(row.Password)
		if err != nil {
			return nil, err
		}
		email := ""
		username := row.Username
		if strings.Contains(row.Username, "@") {
			email = row.Username
			username = ""
		}
		displayName := strings.TrimSpace(row.Name)
		if displayName == "" {
			displayName = row.Username
		}
		if displayName == "" {
			displayName = "Chrome 导入账号"
		}
		_, err = s.infra.DB().ExecContext(ctx, `
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, login_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'email_password', ?, 'unknown', ?, ?)
		`, s.infra.NewID(), match.SiteID, displayName, email, username, encryptedPassword, s.infra.Now(), s.infra.Now())
		if err != nil {
			return nil, err
		}
		imported++
	}
	s.infra.Notify("chrome_passwords_imported", "success", "Chrome 密码 CSV 导入完成", fmt.Sprintf("匹配 %d 条，导入 %d 个账号，跳过 %d 个已存在账号。", len(matches), imported, skippedExisting), "account", "")
	return map[string]interface{}{
		"totalRows":       len(rows),
		"matchedRows":     len(matches),
		"uniqueSiteCount": countUniqueMatchedSites(matches),
		"importedCount":   imported,
		"skippedExisting": skippedExisting,
	}, nil
}

// parseChromePasswordCSV parses a Chrome-exported password CSV into rows.
func parseChromePasswordCSV(csvContent string) ([]chromePasswordRow, error) {
	if len(csvContent) > 8*1024*1024 {
		return nil, fmt.Errorf("CSV 文件太大，请分批导入")
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(csvContent, "\ufeff")))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败：%w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV 没有可导入的密码记录")
	}
	header := map[string]int{}
	for index, name := range records[0] {
		header[strings.ToLower(strings.TrimSpace(name))] = index
	}
	urlIndex, okURL := header["url"]
	usernameIndex, okUsername := header["username"]
	passwordIndex, okPassword := header["password"]
	nameIndex, hasName := header["name"]
	if !okURL || !okUsername || !okPassword {
		return nil, fmt.Errorf("CSV 缺少必要列：url、username、password")
	}

	rows := []chromePasswordRow{}
	for _, record := range records[1:] {
		row := chromePasswordRow{
			URL:      csvField(record, urlIndex),
			Username: csvField(record, usernameIndex),
			Password: csvField(record, passwordIndex),
		}
		if hasName {
			row.Name = csvField(record, nameIndex)
		}
		if row.URL == "" || row.Username == "" || row.Password == "" {
			continue
		}
		rows = append(rows, row)
		if len(rows) >= 5000 {
			return nil, fmt.Errorf("CSV 记录超过 5000 条，请分批导入")
		}
	}
	return rows, nil
}

func csvField(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

// loadPasswordSites reads upstream_sites rows that have a base_url for
// Chrome password matching.
func (s *Service) loadPasswordSites(ctx context.Context) ([]passwordSite, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `SELECT id, name, base_url FROM upstream_sites WHERE COALESCE(base_url,'') <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sites := []passwordSite{}
	for rows.Next() {
		var site passwordSite
		if err := rows.Scan(&site.ID, &site.Name, &site.BaseURL); err != nil {
			return nil, err
		}
		if isExcludedRelaySite(site.Name, site.BaseURL) {
			continue
		}
		site.Host = hostnameForMatch(site.BaseURL)
		if site.Host != "" {
			sites = append(sites, site)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sites, nil
}

// matchChromePasswordRows matches Chrome rows to sites by host.
// To avoid an N+1 query (one accountExists call per row×site pair), we first
// compute the set of (siteID, username) pairs we care about, then issue a
// single batched query to load existing accounts into a lookup map.
func (s *Service) matchChromePasswordRows(ctx context.Context, rows []chromePasswordRow, sites []passwordSite) ([]chromePasswordMatch, error) {
	matches := []chromePasswordMatch{}
	seen := map[string]bool{}
	// First pass: collect all candidate matches without touching the DB, so we
	// know exactly which (siteID, username) pairs we need to look up.
	type candidate struct {
		Site passwordSite
		Row  chromePasswordRow
	}
	candidates := make([]candidate, 0, len(rows)*len(sites))
	for _, row := range rows {
		rowHost := hostnameForMatch(row.URL)
		if rowHost == "" {
			continue
		}
		for _, site := range sites {
			if !hostsMatch(rowHost, site.Host) {
				continue
			}
			key := site.ID + "|" + row.URL + "|" + row.Username
			if seen[key] {
				continue
			}
			seen[key] = true
			candidates = append(candidates, candidate{Site: site, Row: row})
		}
	}
	if len(candidates) == 0 {
		return matches, nil
	}
	// Batch-load existing accounts for the relevant sites/usernames. We query
	// by upstream_site_id IN (...) and then filter the email/username matches
	// in memory, which collapses N*M queries into one.
	siteIDs := make([]interface{}, 0, len(sites))
	siteIDSet := map[string]bool{}
	for _, site := range sites {
		if !siteIDSet[site.ID] {
			siteIDSet[site.ID] = true
			siteIDs = append(siteIDs, site.ID)
		}
	}
	existingAccounts, err := s.loadExistingAccountsForSites(ctx, siteIDs)
	if err != nil {
		return nil, err
	}
	for _, c := range candidates {
		matches = append(matches, chromePasswordMatch{
			SiteID:          c.Site.ID,
			SiteName:        c.Site.Name,
			SiteBaseURL:     c.Site.BaseURL,
			ChromeName:      c.Row.Name,
			URL:             c.Row.URL,
			Username:        c.Row.Username,
			PasswordMasked:  maskSecret(c.Row.Password),
			ExistingAccount: existingAccounts.has(c.Site.ID, c.Row.Username),
		})
	}
	return matches, nil
}

// existingAccountIndex is an in-memory lookup of (siteID, username/email) →
// exists, populated by a single batched query.
type existingAccountIndex struct {
	entries map[string]bool
}

func (idx existingAccountIndex) has(siteID, username string) bool {
	return idx.entries[siteID+"\x00"+username]
}

// loadExistingAccountsForSites runs one IN(...) query per site-list and
// returns an index keyed by "siteID\x00emailOrUsername".
func (s *Service) loadExistingAccountsForSites(ctx context.Context, siteIDs []interface{}) (existingAccountIndex, error) {
	idx := existingAccountIndex{entries: map[string]bool{}}
	if len(siteIDs) == 0 {
		return idx, nil
	}
	placeholders := strings.Repeat("?,", len(siteIDs))
	placeholders = placeholders[:len(placeholders)-1]
	query := `SELECT upstream_site_id, COALESCE(email,''), COALESCE(username,'') FROM channel_accounts WHERE upstream_site_id IN (` + placeholders + `)`
	rows, err := s.infra.DB().QueryContext(ctx, query, siteIDs...)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var siteID, email, username string
		if err := rows.Scan(&siteID, &email, &username); err != nil {
			return idx, err
		}
		if email != "" {
			idx.entries[siteID+"\x00"+email] = true
		}
		if username != "" {
			idx.entries[siteID+"\x00"+username] = true
		}
	}
	if err := rows.Err(); err != nil {
		return idx, err
	}
	return idx, nil
}

// accountExists reports whether an account already exists for the given site
// and username/email. Kept for callers that need a single lookup; the batched
// matchChromePasswordRows path uses loadExistingAccountsForSites instead.
func (s *Service) accountExists(ctx context.Context, siteID string, usernameOrEmail string) (bool, error) {
	var count int
	err := s.infra.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM channel_accounts
		WHERE upstream_site_id = ? AND (email = ? OR username = ?)
	`, siteID, usernameOrEmail, usernameOrEmail).Scan(&count)
	return count > 0, err
}
