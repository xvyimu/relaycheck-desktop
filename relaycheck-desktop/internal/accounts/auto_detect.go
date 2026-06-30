package accounts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AutoDetectSQLiteDBs searches common locations for NewAPI SQLite databases
// that contain a "channels" table.
func (s *Service) AutoDetectSQLiteDBs(ctx context.Context) []string {
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
		if probeSQLiteHasChannels(ctx, absPath) {
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
				if probeSQLiteHasChannels(ctx, absPath) {
					found = append(found, absPath)
				}
			}
		}
	}

	return found
}

// AutoDetectAndImport scans for NewAPI SQLite DBs and imports each one,
// returning per-DB results and an audit-ready summary.
func (s *Service) AutoDetectAndImport(ctx context.Context) (map[string]interface{}, error) {
	dbPaths := s.AutoDetectSQLiteDBs(ctx)
	if len(dbPaths) == 0 {
		return map[string]interface{}{
			"found":   false,
			"message": "未找到可用的 NewAPI SQLite 数据库。",
			"results": []AutoDetectResult{},
		}, nil
	}

	results := []AutoDetectResult{}
	totalImported := 0
	for _, dbPath := range dbPaths {
		baseURL := s.BaseURLForAutoDetectedDB(ctx, dbPath)

		result, err := s.ImportChannelsFromSQLiteWithOptions(ctx, dbPath, false, "", baseURL, true, true, true)
		if err != nil {
			results = append(results, AutoDetectResult{
				DBPath:  dbPath,
				BaseURL: baseURL,
				Error:   err.Error(),
			})
			continue
		}

		imported := intFromResult(result, "importedCount")
		totalImported += imported

		results = append(results, AutoDetectResult{
			DBPath:        dbPath,
			BaseURL:       baseURL,
			ImportedCount: imported,
			SitesCreated:  intFromResult(result, "sitesCreated"),
			SitesMerged:   intFromResult(result, "sitesMerged"),
		})
	}

	s.infra.Audit("auto-detect-import", "info", "", "", "", fmt.Sprintf("自动检测导入完成：发现 %d 个数据库，导入 %d 条渠道。", len(dbPaths), totalImported), map[string]interface{}{
		"dbCount":  len(dbPaths),
		"imported": totalImported,
	})

	return map[string]interface{}{
		"found":   true,
		"message": fmt.Sprintf("自动检测完成：发现 %d 个 NewAPI 数据库（共导入 %d 条渠道）。", len(dbPaths), totalImported),
		"results": results,
	}, nil
}
