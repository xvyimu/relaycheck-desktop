package core

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type AccountSessionCleanupService struct {
	db       *sql.DB
	dataDir  string
	sessions *BrowserSessionStore
	audit    func(action, level, actor, resourceType, resourceID, summary string, metadata map[string]interface{})
}

func NewAccountSessionCleanupService(app *App) *AccountSessionCleanupService {
	return &AccountSessionCleanupService{
		db:       app.db,
		dataDir:  app.dataDir,
		sessions: app.browserSessions,
		audit:    app.audit,
	}
}

func (s *AccountSessionCleanupService) Clear(ctx context.Context, id string) error {
	var profilePath string
	s.sessions.Delete(id)
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(browser_profile_path,'') FROM channel_accounts WHERE id=?`, id).Scan(&profilePath); err != nil && err != sql.ErrNoRows {
		log.Printf("[accounts] clearAccountSession load profile path failed for %s: %v", id, err)
	}
	if profilePath != "" && pathInsideDir(s.dataDir, profilePath) {
		if rmErr := os.RemoveAll(profilePath); rmErr != nil {
			log.Printf("[accounts] clearAccountSession: remove profile %s failed: %v", profilePath, rmErr)
		}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET cookie_encrypted='', browser_profile_path='', user_agent='', login_status='manual_required', updated_at=?
		WHERE id=?
	`, now(), id)
	if err != nil {
		return err
	}
	s.audit("browser_auth.disconnected", "warning", "", "account", id, "网页登录授权已断开。", nil)
	return nil
}

func pathInsideDir(baseDir string, target string) bool {
	baseDir = filepath.Clean(baseDir)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(baseDir, target)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
