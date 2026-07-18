package accounts

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestImportErrorsClassifyUntrustedSources(t *testing.T) {
	t.Run("sqlite path outside allowed roots", func(t *testing.T) {
		volume := filepath.VolumeName(mustWorkingDir(t))
		outside := filepath.Join(volume+string(os.PathSeparator), "relaycheck-denied", "source.db")
		_, err := resolveAllowedSQLiteImportPath(outside)
		if !errors.Is(err, ErrSQLitePathRejected) {
			t.Fatalf("expected ErrSQLitePathRejected, got %v", err)
		}
	})

	t.Run("malformed chrome csv", func(t *testing.T) {
		_, err := parseChromePasswordCSV("name,url\nunterminated,\"")
		if !errors.Is(err, ErrImportInvalidFormat) {
			t.Fatalf("expected ErrImportInvalidFormat, got %v", err)
		}
	})

	t.Run("malformed legacy json", func(t *testing.T) {
		svc := NewService(&stubInfra{})
		_, err := svc.ImportLegacyConfig(context.Background(), `{`, "config.json")
		if !errors.Is(err, ErrImportInvalidFormat) {
			t.Fatalf("expected ErrImportInvalidFormat, got %v", err)
		}
	})
}

func TestAdminAPIImportErrorsSeparateAuthAndAvailability(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		transport  error
		want       error
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, want: ErrImportUpstreamAuth},
		{name: "forbidden", statusCode: http.StatusForbidden, want: ErrImportUpstreamAuth},
		{name: "upstream failure", statusCode: http.StatusInternalServerError, want: ErrImportUpstreamUnavailable},
		{name: "network failure", transport: errors.New("dial token=TOP_SECRET"), want: ErrImportUpstreamUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			infra := &stubInfra{doHTTPFn: func(*http.Request) (*http.Response, error) {
				if tc.transport != nil {
					return nil, tc.transport
				}
				return &http.Response{
					StatusCode: tc.statusCode,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error":"token=TOP_SECRET"}`)),
					Header:     make(http.Header),
				}, nil
			}}
			svc := NewService(infra)
			_, err := svc.fetchAdminAPIChannels(context.Background(), "https://newapi.example", "secret", "1", 0, 100)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestImportErrorsSeparateInvalidSQLiteFromStorageFailure(t *testing.T) {
	t.Run("sqlite without channels table", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("RELAYCHECK_SQLITE_IMPORT_ROOTS", dir)
		sourcePath := filepath.Join(dir, "invalid.db")
		source, err := sql.Open("sqlite", sourcePath)
		if err != nil {
			t.Fatalf("open source: %v", err)
		}
		if _, err := source.Exec(`CREATE TABLE metadata (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatalf("create source: %v", err)
		}
		if err := source.Close(); err != nil {
			t.Fatalf("close source: %v", err)
		}

		db := setupAccountsTestDB(t)
		svc := NewService(&stubInfra{db: db})
		_, err = svc.ImportChannelsFromSQLite(context.Background(), sourcePath, false, "", "", false, false)
		if !errors.Is(err, ErrImportInvalidFormat) {
			t.Fatalf("expected ErrImportInvalidFormat, got %v", err)
		}
		if errors.Is(err, ErrSQLitePathRejected) {
			t.Fatalf("invalid schema was misclassified as path rejection: %v", err)
		}
	})

	t.Run("legacy database failure", func(t *testing.T) {
		db := setupAccountsTestDB(t)
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
		svc := NewService(&stubInfra{db: db})
		_, err := svc.ImportLegacyConfig(context.Background(), `{"base_url":"https://legacy.example"}`, "config.json")
		if !errors.Is(err, ErrImportStorage) {
			t.Fatalf("expected ErrImportStorage, got %v", err)
		}
		if errors.Is(err, ErrImportInvalidFormat) {
			t.Fatalf("storage failure was misclassified as invalid format: %v", err)
		}
	})
}

func mustWorkingDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	return dir
}
