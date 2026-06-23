package core

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type chromePasswordRow struct {
	Name     string
	URL      string
	Username string
	Password string
}

type passwordSite struct {
	ID      string
	Name    string
	BaseURL string
	Host    string
}

type chromePasswordMatch struct {
	SiteID          string `json:"siteId"`
	SiteName        string `json:"siteName"`
	SiteBaseURL     string `json:"siteBaseUrl"`
	ChromeName      string `json:"chromeName"`
	URL             string `json:"url"`
	Username        string `json:"username"`
	PasswordMasked  string `json:"passwordMasked"`
	ExistingAccount bool   `json:"existingAccount"`
}

func (a *App) handleChromePasswordImportPreview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		CSVContent string `json:"csvContent"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.CSVContent) == "" {
		writeError(w, http.StatusBadRequest, "请先选择 Chrome 手动导出的密码 CSV 文件。")
		return
	}
	result, err := a.previewChromePasswordImport(r.Context(), input.CSVContent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleChromePasswordImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	var input struct {
		CSVContent string `json:"csvContent"`
	}
	if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.CSVContent) == "" {
		writeError(w, http.StatusBadRequest, "请先选择 Chrome 手动导出的密码 CSV 文件。")
		return
	}
	result, err := a.importChromePasswords(r.Context(), input.CSVContent)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.audit("import.chrome_passwords", "warning", "", "account", "", "Chrome 密码 CSV 导入完成。", map[string]interface{}{
		"totalRows":       intFromResult(result, "totalRows"),
		"matchedRows":     intFromResult(result, "matchedRows"),
		"uniqueSiteCount": intFromResult(result, "uniqueSiteCount"),
		"importedCount":   intFromResult(result, "importedCount"),
		"skippedExisting": intFromResult(result, "skippedExisting"),
	})
	writeJSON(w, http.StatusOK, result)
}

func (a *App) previewChromePasswordImport(ctx context.Context, csvContent string) (map[string]interface{}, error) {
	rows, err := parseChromePasswordCSV(csvContent)
	if err != nil {
		return nil, err
	}
	sites, err := a.loadPasswordSites(ctx)
	if err != nil {
		return nil, err
	}
	matches, err := a.matchChromePasswordRows(ctx, rows, sites)
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

func (a *App) importChromePasswords(ctx context.Context, csvContent string) (map[string]interface{}, error) {
	rows, err := parseChromePasswordCSV(csvContent)
	if err != nil {
		return nil, err
	}
	sites, err := a.loadPasswordSites(ctx)
	if err != nil {
		return nil, err
	}
	matches, err := a.matchChromePasswordRows(ctx, rows, sites)
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
		encryptedPassword, err := a.encryptText(row.Password)
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
		_, err = a.db.ExecContext(ctx, `
			INSERT INTO channel_accounts (id, upstream_site_id, display_name, email, username, auth_type, password_encrypted, login_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'email_password', ?, 'unknown', ?, ?)
		`, newID(), match.SiteID, displayName, email, username, encryptedPassword, now(), now())
		if err != nil {
			return nil, err
		}
		imported++
	}
	a.notify("chrome_passwords_imported", "success", "Chrome 密码 CSV 导入完成", fmt.Sprintf("匹配 %d 条，导入 %d 个账号，跳过 %d 个已存在账号。", len(matches), imported, skippedExisting), "account", "")
	return map[string]interface{}{
		"totalRows":       len(rows),
		"matchedRows":     len(matches),
		"uniqueSiteCount": countUniqueMatchedSites(matches),
		"importedCount":   imported,
		"skippedExisting": skippedExisting,
	}, nil
}

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

func (a *App) loadPasswordSites(ctx context.Context) ([]passwordSite, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT id, name, base_url FROM upstream_sites WHERE COALESCE(base_url,'') <> ''`)
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
	return sites, nil
}

func (a *App) matchChromePasswordRows(ctx context.Context, rows []chromePasswordRow, sites []passwordSite) ([]chromePasswordMatch, error) {
	matches := []chromePasswordMatch{}
	seen := map[string]bool{}
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
			exists, err := a.accountExists(ctx, site.ID, row.Username)
			if err != nil {
				return nil, err
			}
			matches = append(matches, chromePasswordMatch{
				SiteID:          site.ID,
				SiteName:        site.Name,
				SiteBaseURL:     site.BaseURL,
				ChromeName:      row.Name,
				URL:             row.URL,
				Username:        row.Username,
				PasswordMasked:  maskSecret(row.Password),
				ExistingAccount: exists,
			})
		}
	}
	return matches, nil
}

func (a *App) accountExists(ctx context.Context, siteID string, usernameOrEmail string) (bool, error) {
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM channel_accounts
		WHERE upstream_site_id = ? AND (email = ? OR username = ?)
	`, siteID, usernameOrEmail, usernameOrEmail).Scan(&count)
	return count > 0, err
}

func hostnameForMatch(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + strings.TrimSpace(raw))
		if err != nil {
			return ""
		}
	}
	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	return host
}

func hostsMatch(left string, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.HasSuffix(left, "."+right) || strings.HasSuffix(right, "."+left)
}

func countUniqueMatchedSites(matches []chromePasswordMatch) int {
	seen := map[string]bool{}
	for _, match := range matches {
		seen[match.SiteID] = true
	}
	return len(seen)
}

func findChromeRow(rows []chromePasswordRow, targetURL string, username string) chromePasswordRow {
	for _, row := range rows {
		if row.URL == targetURL && row.Username == username {
			return row
		}
	}
	return chromePasswordRow{}
}
