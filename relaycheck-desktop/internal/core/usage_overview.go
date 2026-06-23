package core

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"time"
)

type usageOverview struct {
	GeneratedAt       string             `json:"generatedAt"`
	AccountCount      int                `json:"accountCount"`
	SiteCount         int                `json:"siteCount"`
	LowBalanceCount   int                `json:"lowBalanceCount"`
	DecliningCount    int                `json:"decliningCount"`
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

func (a *App) handleUsageOverview(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	overview, err := cachedRead(a, "usage-overview", overviewReadCacheTTL, func() (usageOverview, error) {
		return a.buildUsageOverview(r.Context())
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) buildUsageOverview(ctx context.Context) (usageOverview, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT b.account_id, a.display_name, b.upstream_site_id, s.name,
		       b.balance, b.unit, b.created_at
		FROM balance_snapshots b
		JOIN channel_accounts a ON a.id = b.account_id
		JOIN upstream_sites s ON s.id = b.upstream_site_id
		ORDER BY b.account_id ASC, b.created_at DESC
		LIMIT 1000
	`)
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
	sites := map[string]*usageSiteItem{}
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
		overview.Accounts = append(overview.Accounts, item)
	}
	for _, site := range sites {
		overview.Sites = append(overview.Sites, *site)
	}
	overview.AccountCount = len(overview.Accounts)
	overview.SiteCount = len(overview.Sites)
	sort.SliceStable(overview.Accounts, func(i, j int) bool {
		left := overview.Accounts[i]
		right := overview.Accounts[j]
		if left.LowBalance != right.LowBalance {
			return left.LowBalance
		}
		if left.Trend != right.Trend {
			return left.Trend == "down"
		}
		return left.AccountName < right.AccountName
	})
	sort.SliceStable(overview.Sites, func(i, j int) bool {
		left := overview.Sites[i]
		right := overview.Sites[j]
		if left.LowBalanceCount != right.LowBalanceCount {
			return left.LowBalanceCount > right.LowBalanceCount
		}
		return left.SiteName < right.SiteName
	})
	overview.Accounts = limitUsageAccountItems(overview.Accounts, 80)
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
