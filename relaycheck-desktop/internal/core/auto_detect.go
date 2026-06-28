package core

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var defaultNewAPISearchPaths = []string{
	`D:\newapi\data\one-api.db`,
	`D:\new-api\data\one-api.db`,
	`one-api.db`,
	`data\one-api.db`,
}

var defaultNewAPISearchDirs = []string{
	`D:\newapi`,
	`D:\new-api`,
}

// baseURLFromDBPath maps common DB paths to their likely NewAPI base URL.
func baseURLFromDBPath(dbPath string) string {
	normalized := strings.ToLower(filepath.ToSlash(dbPath))
	if strings.Contains(normalized, "newapi") || strings.Contains(normalized, "new-api") || strings.Contains(normalized, "one-api") {
		return "http://127.0.0.1:3000"
	}
	return ""
}

func (a *App) baseURLForAutoDetectedDB(ctx context.Context, dbPath string) string {
	cleanPath, err := filepath.Abs(dbPath)
	if err != nil {
		cleanPath = dbPath
	}
	var baseURL string
	err = a.db.QueryRowContext(ctx, `
		SELECT base_url
		FROM local_newapi_instances
		WHERE database_path=?
		ORDER BY updated_at DESC
		LIMIT 1
	`, cleanPath).Scan(&baseURL)
	if err == nil && strings.TrimSpace(baseURL) != "" {
		return baseURL
	}
	return baseURLFromDBPath(cleanPath)
}

type autoDetectResult struct {
	DBPath        string `json:"dbPath"`
	BaseURL       string `json:"baseUrl"`
	ImportedCount int    `json:"importedCount"`
	SitesCreated  int    `json:"sitesCreated"`
	SitesMerged   int    `json:"sitesMerged"`
	Error         string `json:"error,omitempty"`
}

// autoDetectSQLiteDBs searches common locations for NewAPI SQLite databases
// that contain a "channels" table.
func (a *App) autoDetectSQLiteDBs(ctx context.Context) []string {
	var found []string
	seen := map[string]bool{}

	// 1. Check exact known paths
	for _, path := range defaultNewAPISearchPaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if seen[absPath] {
			continue
		}
		seen[absPath] = true
		if _, err := os.Stat(absPath); err != nil {
			continue
		}
		if a.probeSQLiteHasChannels(ctx, absPath) {
			found = append(found, absPath)
		}
	}

	// 2. Scan known parent directories for *.db files in data/
	for _, dir := range defaultNewAPISearchDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		for _, sub := range []string{"data"} {
			dataDir := filepath.Join(absDir, sub)
			entries, err := os.ReadDir(dataDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if !strings.HasSuffix(strings.ToLower(entry.Name()), ".db") {
					continue
				}
				absPath := filepath.Join(dataDir, entry.Name())
				if seen[absPath] {
					continue
				}
				seen[absPath] = true
				if a.probeSQLiteHasChannels(ctx, absPath) {
					found = append(found, absPath)
				}
			}
		}
	}

	return found
}

// probeSQLiteHasChannels checks if the given SQLite DB has a "channels" table.
func (a *App) probeSQLiteHasChannels(ctx context.Context, dbPath string) bool {
	source, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return false
	}
	defer source.Close()

	var tableName string
	err = source.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='channels'`).Scan(&tableName)
	return err == nil && tableName == "channels"
}

func (a *App) handleAutoDetectAndImport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	dbPaths := a.autoDetectSQLiteDBs(r.Context())
	if len(dbPaths) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"found":   false,
			"message": "未找到可用的 NewAPI SQLite 数据库。",
			"results": []autoDetectResult{},
		})
		return
	}

	results := []autoDetectResult{}
	totalImported := 0
	for _, dbPath := range dbPaths {
		baseURL := a.baseURLForAutoDetectedDB(r.Context(), dbPath)

		result, err := a.importChannelsFromSQLiteWithOptions(r.Context(), dbPath, false, "", baseURL, true, true, true)
		if err != nil {
			results = append(results, autoDetectResult{
				DBPath:  dbPath,
				BaseURL: baseURL,
				Error:   err.Error(),
			})
			continue
		}

		imported := intFromResult(result, "importedCount")
		totalImported += imported

		results = append(results, autoDetectResult{
			DBPath:        dbPath,
			BaseURL:       baseURL,
			ImportedCount: imported,
			SitesCreated:  intFromResult(result, "sitesCreated"),
			SitesMerged:   intFromResult(result, "sitesMerged"),
		})
	}

	a.audit("auto-detect-import", "info", "", "", "", fmt.Sprintf("自动检测导入完成：发现 %d 个数据库，导入 %d 条渠道。", len(dbPaths), totalImported), map[string]interface{}{
		"dbCount":  len(dbPaths),
		"imported": totalImported,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"found":   true,
		"message": fmt.Sprintf("自动检测完成：发现 %d 个 NewAPI 数据库（共导入 %d 条渠道）。", len(dbPaths), totalImported),
		"results": results,
	})
}
