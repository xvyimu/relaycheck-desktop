package core

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LegacyPythonCheckResult reports the status of legacy Python code checks.
type LegacyPythonCheckResult struct {
	LegacyDirExists   bool     `json:"legacyDirExists"`
	APIRoutesCount    int      `json:"apiRoutesCount"`
	DBInitTables      int      `json:"dbInitTables"`
	DBInitIdempotent  bool     `json:"dbInitIdempotent"`
	CheckedAt         string   `json:"checkedAt"`
	Notes             []string `json:"notes,omitempty"`
}

func (a *App) handleLegacyPythonCheck(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}

	result := LegacyPythonCheckResult{
		CheckedAt: now(),
		Notes:     []string{},
	}

	// Search for legacy Python directory in common archive locations.
	// Tries: <dataDir>/../_archive/*/legacy/newapi_signin,
	//       <dataDir>/../_archive/legacy/newapi_signin,
	//       <dataDir>/../legacy/newapi_signin
	candidates := []string{
		filepath.Join(a.dataDir, "..", "_archive", "legacy", "newapi_signin"),
		filepath.Join(a.dataDir, "..", "legacy", "newapi_signin"),
		filepath.Join(a.dataDir, "..", "..", "newapi_signin"),
	}
	// Also glob for date-stamped archive directories
	if matches, err := filepath.Glob(filepath.Join(a.dataDir, "..", "_archive", "*", "legacy", "newapi_signin")); err == nil {
		candidates = append(candidates, matches...)
	}

	var legacyDir string
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "api.py")); err == nil {
			legacyDir = candidate
			break
		}
	}

	if legacyDir == "" {
		result.Notes = append(result.Notes, "未找到遗留 Python 目录，跳过所有检查")
		writeJSON(w, http.StatusOK, result)
		return
	}

	apiPath := filepath.Join(legacyDir, "api.py")
	dbPath := filepath.Join(legacyDir, "database.py")

	if _, err := os.Stat(apiPath); err != nil {
		result.Notes = append(result.Notes, "未找到遗留 Python api.py，跳过路由检查")
	} else {
		content, err := os.ReadFile(apiPath)
		if err == nil {
			// Count @app.route decorators
			routeRegex := regexp.MustCompile(`@app\.route\(`)
			matches := routeRegex.FindAllString(string(content), -1)
			result.APIRoutesCount = len(matches)
			result.LegacyDirExists = true
			result.Notes = append(result.Notes, "已检查遗留 Python api.py 路由数")
		}
	}

	if _, err := os.Stat(dbPath); err != nil {
		result.Notes = append(result.Notes, "未找到遗留 Python database.py，跳过 DB init 检查")
	} else {
		content, err := os.ReadFile(dbPath)
		if err == nil {
			// Count CREATE TABLE statements
			tableRegex := regexp.MustCompile(`CREATE TABLE[^;]+`)
			matches := tableRegex.FindAllString(string(content), -1)
			result.DBInitTables = len(matches)

			// Check for IF NOT EXISTS (idempotency indicator)
			if strings.Contains(string(content), "IF NOT EXISTS") {
				result.DBInitIdempotent = true
				result.Notes = append(result.Notes, "遗留 Python DB init 使用 IF NOT EXISTS，幂等")
			} else {
				result.DBInitIdempotent = false
				result.Notes = append(result.Notes, "遗留 Python DB init 未使用 IF NOT EXISTS，非幂等")
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}
