package core

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	accountCleanupPreviewTTL         = 5 * time.Minute
	maxPendingAccountCleanupPreviews = 64
)

var (
	errAccountCleanupPreviewCapacity    = errors.New("account cleanup preview capacity reached")
	errAccountCleanupPreviewUnavailable = errors.New("account cleanup preview unavailable")
	errAccountCleanupPreviewStale       = errors.New("account cleanup preview stale")
)

type accountCleanupPreview struct {
	ID                     string
	CreatedAt              time.Time
	ExpiresAt              time.Time
	Limit                  int
	HasMore                bool
	IncludeLastUnsupported bool
	Items                  []unsupportedCheckinAccountItem
}

type AccountCleanupService struct {
	db                  *sql.DB
	invalidateReadCache func()
	dbMu                sync.RWMutex
	previewMu           sync.Mutex
	previews            map[string]accountCleanupPreview
	now                 func() time.Time
}

// NewAccountCleanupService 创建账号清理服务，并初始化仅驻留进程内的短期预览存储。
func NewAccountCleanupService(app *App) *AccountCleanupService {
	return &AccountCleanupService{
		db:                  app.db,
		invalidateReadCache: app.invalidateReadCache,
		previews:            map[string]accountCleanupPreview{},
		now:                 time.Now,
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
	PreviewID              string                          `json:"previewId,omitempty"`
	ExpiresAt              string                          `json:"expiresAt,omitempty"`
	Matched                int                             `json:"matched"`
	Deleted                int                             `json:"deleted"`
	Limit                  int                             `json:"limit"`
	HasMore                bool                            `json:"hasMore"`
	DryRun                 bool                            `json:"dryRun"`
	IncludeLastUnsupported bool                            `json:"includeLastUnsupported"`
	Items                  []unsupportedCheckinAccountItem `json:"items"`
}

// PreviewUnsupportedCheckinAccounts 读取候选并冻结一次性预览；没有候选时不签发 previewId。
func (s *AccountCleanupService) PreviewUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool) (unsupportedCheckinCleanupResult, error) {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()

	limit = clampBatchLimit(limit, 10)
	items, hasMore, err := loadUnsupportedCheckinAccounts(ctx, s.db, limit, includeLastUnsupported, nil)
	if err != nil {
		return unsupportedCheckinCleanupResult{}, err
	}
	result := unsupportedCheckinCleanupResult{
		Matched:                len(items),
		Limit:                  limit,
		HasMore:                hasMore,
		DryRun:                 true,
		IncludeLastUnsupported: includeLastUnsupported,
		Items:                  items,
	}
	if len(items) == 0 {
		return result, nil
	}

	createdAt := s.now().UTC()
	preview := accountCleanupPreview{
		ID:                     newID(),
		CreatedAt:              createdAt,
		ExpiresAt:              createdAt.Add(accountCleanupPreviewTTL),
		Limit:                  limit,
		HasMore:                hasMore,
		IncludeLastUnsupported: includeLastUnsupported,
		Items:                  items,
	}
	// 预览写入有界内存存储，避免把待删除账号或令牌持久化到数据库。
	if err := s.putPreview(preview); err != nil {
		return unsupportedCheckinCleanupResult{}, err
	}
	result.PreviewID = preview.ID
	result.ExpiresAt = preview.ExpiresAt.Format(time.RFC3339Nano)
	return result, nil
}

// ConfirmUnsupportedCheckinAccounts 一次性消费预览，并只删除冻结且仍满足原条件的账号。
func (s *AccountCleanupService) ConfirmUnsupportedCheckinAccounts(ctx context.Context, previewID string) (unsupportedCheckinCleanupResult, error) {
	// 读锁覆盖 Claim 和整个事务，避免恢复数据库时把旧 token 用到新数据集。
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()

	// Claim 在事务前原子消费 previewId，确保并发确认或重放最多只有一个成功者。
	preview, err := s.claimPreview(previewID)
	if err != nil {
		return unsupportedCheckinCleanupResult{}, err
	}

	result := unsupportedCheckinCleanupResult{
		Matched:                len(preview.Items),
		Limit:                  preview.Limit,
		HasMore:                preview.HasMore,
		DryRun:                 false,
		IncludeLastUnsupported: preview.IncludeLastUnsupported,
		Items:                  cloneUnsupportedCheckinItems(preview.Items),
	}
	if len(preview.Items) == 0 {
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

	ids := unsupportedCheckinAccountIDs(preview.Items)
	// 在同一事务内复核完整冻结集合；任一账号缺失或不再符合条件都整批失败。
	current, _, err := loadUnsupportedCheckinAccounts(ctx, tx, len(ids), preview.IncludeLastUnsupported, ids)
	if err != nil {
		return result, err
	}
	if !sameUnsupportedCheckinAccounts(ids, current) {
		return result, errAccountCleanupPreviewStale
	}

	args := stringsToInterfaces(ids)
	placeholders := sqlPlaceholders(len(ids))
	// 三次删除共享同一事务；账号最终删除数量不精确时会回滚前两次关联数据删除。
	if _, err := tx.ExecContext(ctx, `DELETE FROM checkin_logs WHERE account_id IN (`+placeholders+`)`, args...); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM balance_snapshots WHERE account_id IN (`+placeholders+`)`, args...); err != nil {
		return result, err
	}
	eligibility := unsupportedCheckinEligibilitySQL(preview.IncludeLastUnsupported, "channel_accounts")
	deleted, err := tx.ExecContext(ctx, `
		DELETE FROM channel_accounts
		WHERE id IN (`+placeholders+`)
		  AND EXISTS (
			SELECT 1 FROM upstream_sites s
			WHERE s.id = channel_accounts.upstream_site_id
			  AND `+eligibility+`
		  )
	`, args...)
	if err != nil {
		return result, err
	}
	affected, err := deleted.RowsAffected()
	if err != nil {
		return result, err
	}
	if affected != int64(len(ids)) {
		return result, errAccountCleanupPreviewStale
	}
	result.Deleted = int(affected)
	if err := tx.Commit(); err != nil {
		return result, err
	}
	committed = true
	if s.invalidateReadCache != nil {
		// 仅在事务成功提交后失效读缓存，失败路径保留原视图。
		s.invalidateReadCache()
	}
	return result, nil
}

// DeleteUnsupportedCheckinAccounts 保留内部便捷调用，并在一次调用内安全地预览后立即消费。
func (s *AccountCleanupService) DeleteUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool, dryRun bool) (unsupportedCheckinCleanupResult, error) {
	preview, err := s.PreviewUnsupportedCheckinAccounts(ctx, limit, includeLastUnsupported)
	if err != nil || dryRun || preview.PreviewID == "" {
		return preview, err
	}
	return s.ConfirmUnsupportedCheckinAccounts(ctx, preview.PreviewID)
}

// LoadUnsupportedCheckinAccounts 返回当前范围内的候选，用于生成新的只读预览。
func (s *AccountCleanupService) LoadUnsupportedCheckinAccounts(ctx context.Context, limit int, includeLastUnsupported bool) ([]unsupportedCheckinAccountItem, bool, error) {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return loadUnsupportedCheckinAccounts(ctx, s.db, limit, includeLastUnsupported, nil)
}

// RebindDB 在数据库恢复后切换连接，并原子清空所有绑定旧数据集的破坏性预览。
func (s *AccountCleanupService) RebindDB(db *sql.DB) {
	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.db = db
	s.previews = map[string]accountCleanupPreview{}
}

type accountCleanupQueryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

// loadUnsupportedCheckinAccounts 统一预览和事务复核查询；accountIDs 非空时严禁扩展到冻结集合之外。
func loadUnsupportedCheckinAccounts(ctx context.Context, queryer accountCleanupQueryer, limit int, includeLastUnsupported bool, accountIDs []string) ([]unsupportedCheckinAccountItem, bool, error) {
	limit = clampBatchLimit(limit, 10)
	where := unsupportedCheckinEligibilitySQL(includeLastUnsupported, "a")
	args := make([]interface{}, 0, len(accountIDs)+1)
	if len(accountIDs) > 0 {
		where += ` AND a.id IN (` + sqlPlaceholders(len(accountIDs)) + `)`
		args = append(args, stringsToInterfaces(accountIDs)...)
	}
	args = append(args, limit+1)
	rows, err := queryer.QueryContext(ctx, `
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
	`, args...)
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

// unsupportedCheckinEligibilitySQL 生成预览与确认共用的资格谓词，避免两条路径口径漂移。
func unsupportedCheckinEligibilitySQL(includeLastUnsupported bool, accountAlias string) string {
	where := `s.supports_checkin = 0`
	if includeLastUnsupported {
		where = `(` + where + ` OR lower(COALESCE(` + accountAlias + `.last_checkin_status,'')) = 'unsupported')`
	}
	return where
}

// unsupportedCheckinAccountIDs 按预览顺序提取冻结账号 ID。
func unsupportedCheckinAccountIDs(items []unsupportedCheckinAccountItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.AccountID)
	}
	return ids
}

// sameUnsupportedCheckinAccounts 验证事务内候选与冻结 ID 完全一致，不依赖查询排序。
func sameUnsupportedCheckinAccounts(expectedIDs []string, items []unsupportedCheckinAccountItem) bool {
	if len(expectedIDs) != len(items) {
		return false
	}
	actual := make(map[string]struct{}, len(items))
	for _, item := range items {
		actual[item.AccountID] = struct{}{}
	}
	if len(actual) != len(expectedIDs) {
		return false
	}
	for _, id := range expectedIDs {
		if _, ok := actual[id]; !ok {
			return false
		}
	}
	return true
}

// sqlPlaceholders 为已校验的固定数量参数生成 SQLite 占位符。
func sqlPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

// stringsToInterfaces 将字符串参数转换为 database/sql 所需的可变参数切片。
func stringsToInterfaces(values []string) []interface{} {
	args := make([]interface{}, len(values))
	for index, value := range values {
		args[index] = value
	}
	return args
}

// cloneUnsupportedCheckinItems 复制候选切片，防止调用方修改存储中的冻结计划。
func cloneUnsupportedCheckinItems(items []unsupportedCheckinAccountItem) []unsupportedCheckinAccountItem {
	return append([]unsupportedCheckinAccountItem(nil), items...)
}

// putPreview 写入短期预览，并在达到容量上限时 fail-closed。
func (s *AccountCleanupService) putPreview(preview accountCleanupPreview) error {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.purgeExpiredPreviewsLocked(s.now())
	if len(s.previews) >= maxPendingAccountCleanupPreviews {
		return errAccountCleanupPreviewCapacity
	}
	preview.Items = cloneUnsupportedCheckinItems(preview.Items)
	s.previews[preview.ID] = preview
	return nil
}

// claimPreview 原子消费未过期预览，保证 token 只能使用一次。
func (s *AccountCleanupService) claimPreview(previewID string) (accountCleanupPreview, error) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	nowTime := s.now()
	s.purgeExpiredPreviewsLocked(nowTime)
	preview, ok := s.previews[strings.TrimSpace(previewID)]
	if !ok || !preview.ExpiresAt.After(nowTime) {
		return accountCleanupPreview{}, errAccountCleanupPreviewUnavailable
	}
	delete(s.previews, preview.ID)
	preview.Items = cloneUnsupportedCheckinItems(preview.Items)
	return preview, nil
}

// purgeExpiredPreviewsLocked 清理过期预览；调用方必须持有 previewMu。
func (s *AccountCleanupService) purgeExpiredPreviewsLocked(nowTime time.Time) {
	for id, preview := range s.previews {
		if !preview.ExpiresAt.After(nowTime) {
			delete(s.previews, id)
		}
	}
}
