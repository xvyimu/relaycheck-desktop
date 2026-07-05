package core

import (
	"context"
	"database/sql"
	"strings"
)

type AccountCleanupService struct {
	db                  *sql.DB
	invalidateReadCache func()
}

func NewAccountCleanupService(app *App) *AccountCleanupService {
	return &AccountCleanupService{
		db:                  app.db,
		invalidateReadCache: app.invalidateReadCache,
	}
}

type unsupportedCheckinAccountItem struct {
	AccountID         string `json:"accountId"`
	AccountName       string `json:"accountName"`
	UpstreamSiteID    string `json:"upstreamSiteId"`
	UpstreamSiteName  string `json:"upstreamSiteName"`
	UpstreamSiteKind  string `json:"upstreamSiteKind"`
	LastCheckinStatus string `json:"lastCheckinStatus,omitempty"`
	Reason            string `json:"reason"`
}

type unsupportedCheckinCleanupResult struct {
	Matched                int                             `json:"matched"`
	Deleted                int                             `json:"deleted"`
	Limit                  int                             `json:"limit"`
	HasMore                bool                            `json:"hasMore"`
	DryRun                 bool                            `json:"dryRun"`
	IncludeLastUnsupported bool                            `json:"includeLastUnsupported"`
	Items                  []unsupportedCheckinAccountItem `json:"items"`
}

func (s *AccountCleanupService) DeleteUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool, dryRun bool) (unsupportedCheckinCleanupResult, error) {
	limit = clampBatchLimit(limit, 10)
	items, hasMore, err := s.LoadUnsupportedCheckinAccounts(ctx, limit, includeLastUnsupported)
	if err != nil {
		return unsupportedCheckinCleanupResult{}, err
	}
	result := unsupportedCheckinCleanupResult{
		Matched:                len(items),
		Limit:                  limit,
		HasMore:                hasMore,
		DryRun:                 dryRun,
		IncludeLastUnsupported: includeLastUnsupported,
		Items:                  items,
	}
	if dryRun || len(items) == 0 {
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	ids := make([]interface{}, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.AccountID)
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	if _, err := tx.ExecContext(ctx, `DELETE FROM checkin_logs WHERE account_id IN (`+placeholders+`)`, ids...); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM balance_snapshots WHERE account_id IN (`+placeholders+`)`, ids...); err != nil {
		return result, err
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM channel_accounts WHERE id IN (`+placeholders+`)`, ids...)
	if err != nil {
		return result, err
	}
	if affected, _ := deleted.RowsAffected(); affected > 0 {
		result.Deleted = int(affected)
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	if s.invalidateReadCache != nil {
		s.invalidateReadCache()
	}
	return result, nil
}

func (s *AccountCleanupService) LoadUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool) ([]unsupportedCheckinAccountItem, bool, error) {
	limit = clampBatchLimit(limit, 10)
	where := `s.supports_checkin = 0`
	if includeLastUnsupported {
		where = `(` + where + ` OR lower(COALESCE(a.last_checkin_status,'')) = 'unsupported')`
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.display_name, s.id, s.name, s.kind, COALESCE(a.last_checkin_status,''),
		       CASE
		         WHEN s.supports_checkin = 0 THEN 'site_not_support_checkin'
		         ELSE 'last_checkin_unsupported'
		       END
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE `+where+`
		ORDER BY CASE WHEN s.supports_checkin = 0 THEN 0 ELSE 1 END, a.updated_at DESC
		LIMIT ?
	`, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	items := []unsupportedCheckinAccountItem{}
	for rows.Next() {
		var item unsupportedCheckinAccountItem
		if err := rows.Scan(&item.AccountID, &item.AccountName, &item.UpstreamSiteID, &item.UpstreamSiteName, &item.UpstreamSiteKind, &item.LastCheckinStatus, &item.Reason); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}
