package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type usageOverview struct {
	GeneratedAt       string             `json:"generatedAt"`
	AccountCount      int                `json:"accountCount"`
	SiteCount         int                `json:"siteCount"`
	LowBalanceCount   int                `json:"lowBalanceCount"`
	DecliningCount    int                `json:"decliningCount"`
	Truncated         bool               `json:"truncated"`
	EstimatedDailyUse map[string]float64 `json:"estimatedDailyUse"`
	Sites             []usageSiteItem    `json:"sites"`
	Accounts          []usageAccountItem `json:"accounts"`
}

type usageSiteItem struct {
	SiteID            string             `json:"siteId"`
	SiteName          string             `json:"siteName"`
	AccountCount      int                `json:"accountCount"`
	LowBalanceCount   int                `json:"lowBalanceCount"`
	DecliningCount    int                `json:"decliningCount"`
	BalanceByUnit     map[string]float64 `json:"balanceByUnit"`
	EstimatedDailyUse map[string]float64 `json:"estimatedDailyUse"`
}

type usageAccountItem struct {
	AccountID          string   `json:"accountId"`
	AccountName        string   `json:"accountName"`
	SiteID             string   `json:"siteId"`
	SiteName           string   `json:"siteName"`
	Balance            *float64 `json:"balance,omitempty"`
	PreviousBalance    *float64 `json:"previousBalance,omitempty"`
	BalanceDelta       *float64 `json:"balanceDelta,omitempty"`
	Unit               string   `json:"unit"`
	EstimatedDailyUse  *float64 `json:"estimatedDailyUse,omitempty"`
	LowBalance         bool     `json:"lowBalance"`
	Trend              string   `json:"trend"`
	LastSnapshotAt     string   `json:"lastSnapshotAt,omitempty"`
	PreviousSnapshotAt string   `json:"previousSnapshotAt,omitempty"`
}

type usageSnapshotRow struct {
	AccountID   string
	AccountName string
	SiteID      string
	SiteName    string
	Balance     sql.NullFloat64
	Unit        string
	CreatedAt   string
}

type usageOverviewOptions struct {
	SiteID string
	Limit  int
}

func parseUsageOverviewOptions(r *http.Request) (usageOverviewOptions, error) {
	options := usageOverviewOptions{
		SiteID: strings.TrimSpace(r.URL.Query().Get("siteId")),
		Limit:  clampListLimit(queryInt(r, "limit", 80), 80, 200),
	}
	if len(options.SiteID) > 200 {
		return options, errors.New("站点筛选值不能超过 200 个字符。")
	}
	return options, nil
}

func (a *App) handleUsageOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	options, err := parseUsageOverviewOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cacheKey := fmt.Sprintf("usage-overview:%s:%d", options.SiteID, options.Limit)
	overview, err := cachedRead(a, cacheKey, overviewReadCacheTTL, func() (usageOverview, error) {
		return a.buildUsageOverviewWithOptions(r.Context(), options)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) buildUsageOverview(ctx context.Context) (usageOverview, error) {
	return a.buildUsageOverviewWithOptions(ctx, usageOverviewOptions{Limit: 80})
}

func (a *App) buildUsageOverviewWithOptions(ctx context.Context, options usageOverviewOptions) (usageOverview, error) {
	started := time.Now()
	defer func() { logSlowOperation(ctx, "sql", "usage-overview", started, nil, nil) }()
	if options.Limit <= 0 {
		options.Limit = 80
	}
	// Limit target accounts before ranking snapshots so history growth does not
	// increase the window query's working set without bound.
	rows, err := a.db.QueryContext(ctx, `
		WITH target_accounts AS (
			SELECT a.id, a.display_name, a.upstream_site_id, s.name AS site_name
			FROM channel_accounts a
			JOIN upstream_sites s ON s.id = a.upstream_site_id
			WHERE (? = '' OR a.upstream_site_id = ?)
			  AND EXISTS (SELECT 1 FROM balance_snapshots b WHERE b.account_id = a.id)
			ORDER BY a.updated_at DESC, a.id DESC
			LIMIT ?
		), ranked_snapshots AS (
			SELECT b.account_id AS account_id,
			       ta.display_name AS display_name,
			       b.upstream_site_id AS upstream_site_id,
			       ta.site_name AS site_name,
			       b.balance AS balance,
			       b.unit AS unit,
			       b.created_at AS created_at,
			       ROW_NUMBER() OVER (
					PARTITION BY b.account_id
					ORDER BY b.created_at DESC, b.id DESC
			       ) AS rn
			FROM target_accounts ta
			JOIN balance_snapshots b ON b.account_id = ta.id
		)
		SELECT account_id, display_name, upstream_site_id, site_name, balance, unit, created_at
		FROM ranked_snapshots
		WHERE rn <= 2
		ORDER BY account_id ASC, created_at DESC
	`, options.SiteID, options.SiteID, options.Limit+1)
	if err != nil {
		return usageOverview{}, err
	}
	defer rows.Close()

	byAccount := map[string][]usageSnapshotRow{}
	for rows.Next() {
		var row usageSnapshotRow
		if err := rows.Scan(&row.AccountID, &row.AccountName, &row.SiteID, &row.SiteName, &row.Balance, &row.Unit, &row.CreatedAt); err != nil {
			return usageOverview{}, err
		}
		byAccount[row.AccountID] = append(byAccount[row.AccountID], row)
	}
	if err := rows.Err(); err != nil {
		return usageOverview{}, err
	}

	overview := usageOverview{
		GeneratedAt:       now(),
		EstimatedDailyUse: map[string]float64{},
		Sites:             []usageSiteItem{},
		Accounts:          []usageAccountItem{},
	}
	accountItems := make([]usageAccountItem, 0, len(byAccount))
	for _, snapshots := range byAccount {
		if len(snapshots) == 0 {
			continue
		}
		latest := snapshots[0]
		item := usageAccountItem{
			AccountID:      latest.AccountID,
			AccountName:    latest.AccountName,
			SiteID:         latest.SiteID,
			SiteName:       latest.SiteName,
			Balance:        nullableFloat(latest.Balance),
			Unit:           firstNonEmpty(latest.Unit, "unknown"),
			LastSnapshotAt: latest.CreatedAt,
			Trend:          "flat",
			LowBalance:     isLowBalance(latest.Balance, latest.Unit),
		}
		if len(snapshots) > 1 {
			previous := snapshots[1]
			item.PreviousBalance = nullableFloat(previous.Balance)
			item.PreviousSnapshotAt = previous.CreatedAt
			if item.Balance != nil && item.PreviousBalance != nil {
				delta := *item.Balance - *item.PreviousBalance
				item.BalanceDelta = &delta
				item.Trend = usageTrend(delta)
				if daily := estimateDailyUse(*item.PreviousBalance, *item.Balance, previous.CreatedAt, latest.CreatedAt); daily > 0 {
					item.EstimatedDailyUse = &daily
				}
			}
		}
		accountItems = append(accountItems, item)
	}
	sort.SliceStable(accountItems, func(i, j int) bool {
		left := accountItems[i]
		right := accountItems[j]
		if left.LowBalance != right.LowBalance {
			return left.LowBalance
		}
		if left.Trend != right.Trend {
			return left.Trend == "down"
		}
		return left.AccountName < right.AccountName
	})
	if len(accountItems) > options.Limit {
		overview.Truncated = true
		accountItems = accountItems[:options.Limit]
	}
	overview.Accounts = accountItems
	sites := map[string]*usageSiteItem{}
	for _, item := range overview.Accounts {
		site := sites[item.SiteID]
		if site == nil {
			site = &usageSiteItem{
				SiteID:            item.SiteID,
				SiteName:          item.SiteName,
				BalanceByUnit:     map[string]float64{},
				EstimatedDailyUse: map[string]float64{},
			}
			sites[item.SiteID] = site
		}
		site.AccountCount++
		if item.LowBalance {
			site.LowBalanceCount++
			overview.LowBalanceCount++
		}
		if item.Trend == "down" {
			site.DecliningCount++
			overview.DecliningCount++
		}
		if item.Balance != nil {
			site.BalanceByUnit[item.Unit] += *item.Balance
		}
		if item.EstimatedDailyUse != nil {
			site.EstimatedDailyUse[item.Unit] += *item.EstimatedDailyUse
			overview.EstimatedDailyUse[item.Unit] += *item.EstimatedDailyUse
		}
	}
	for _, site := range sites {
		overview.Sites = append(overview.Sites, *site)
	}
	overview.AccountCount = len(overview.Accounts)
	overview.SiteCount = len(overview.Sites)
	sort.SliceStable(overview.Sites, func(i, j int) bool {
		left := overview.Sites[i]
		right := overview.Sites[j]
		if left.LowBalanceCount != right.LowBalanceCount {
			return left.LowBalanceCount > right.LowBalanceCount
		}
		return left.SiteName < right.SiteName
	})
	overview.Sites = limitUsageSiteItems(overview.Sites, 40)
	return overview, nil
}

func estimateDailyUse(previous float64, latest float64, previousAt string, latestAt string) float64 {
	if latest >= previous {
		return 0
	}
	left, errLeft := time.Parse(time.RFC3339Nano, previousAt)
	right, errRight := time.Parse(time.RFC3339Nano, latestAt)
	if errLeft != nil || errRight != nil || !right.After(left) {
		return 0
	}
	days := right.Sub(left).Hours() / 24
	if days <= 0 {
		return 0
	}
	return (previous - latest) / days
}

func usageTrend(delta float64) string {
	switch {
	case delta < -0.000001:
		return "down"
	case delta > 0.000001:
		return "up"
	default:
		return "flat"
	}
}

func isLowBalance(balance sql.NullFloat64, unit string) bool {
	if !balance.Valid {
		return false
	}
	switch unit {
	case "usd", "cny", "unknown":
		return balance.Float64 <= 5
	case "quota":
		return balance.Float64 <= 500000
	case "token":
		return balance.Float64 <= 100000
	default:
		return balance.Float64 <= 5
	}
}

func limitUsageAccountItems(values []usageAccountItem, limit int) []usageAccountItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func limitUsageSiteItems(values []usageSiteItem, limit int) []usageSiteItem {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
