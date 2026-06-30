package legacycheck

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Infra is the subset of the host application that the legacy-check domain
// depends on. The host (e.g. *core.App) satisfies this interface by providing
// the application data directory, which anchors the search for archived
// Python source directories.
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DataDir returns the application data directory path. The legacy
	// Python directory is searched for in archive locations relative to
	// the parent of DataDir.
	DataDir() string
}

// Service implements the legacy-Python-check domain. It scans archive
// locations for a legacy newapi_signin Python directory, then inspects its
// api.py and database.py files to report route counts and DB-init
// idempotency. The host application delegates its *App handler methods to
// this Service.
type Service struct {
	infra Infra
}

// NewService constructs a legacy-check Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}

// LegacyPythonCheckResult reports the status of legacy Python code checks.
type LegacyPythonCheckResult struct {
	LegacyDirExists  bool     `json:"legacyDirExists"`
	APIRoutesCount   int      `json:"apiRoutesCount"`
	DBInitTables     int      `json:"dbInitTables"`
	DBInitIdempotent bool     `json:"dbInitIdempotent"`
	CheckedAt        string   `json:"checkedAt"`
	Notes            []string `json:"notes,omitempty"`
}

// Check performs the legacy Python code inspection and returns the assembled
// result. When no legacy directory is found, the result is returned with a
// note and zeroed counters, mirroring the original handler behaviour where
// every path responds HTTP 200.
func (s *Service) Check(ctx context.Context) *LegacyPythonCheckResult {
	result := &LegacyPythonCheckResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Notes:     []string{},
	}

	// Search for legacy Python directory in common archive locations.
	// Tries: <dataDir>/../_archive/*/legacy/newapi_signin,
	//       <dataDir>/../_archive/legacy/newapi_signin,
	//       <dataDir>/../legacy/newapi_signin
	dataDir := s.infra.DataDir()
	candidates := []string{
		filepath.Join(dataDir, "..", "_archive", "legacy", "newapi_signin"),
		filepath.Join(dataDir, "..", "legacy", "newapi_signin"),
		filepath.Join(dataDir, "..", "..", "newapi_signin"),
	}
	// Also glob for date-stamped archive directories
	if matches, err := filepath.Glob(filepath.Join(dataDir, "..", "_archive", "*", "legacy", "newapi_signin")); err == nil {
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
		return result
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

	return result
}
