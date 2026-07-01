package core

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"
)

// BalanceTrendPoint represents one data point in a balance trend chart.
type BalanceTrendPoint struct {
	Date    string   `json:"date"`
	Balance *float64 `json:"balance,omitempty"`
	Account string   `json:"account,omitempty"`
}

// CheckinDistributionItem represents one slice in a checkin status pie chart.
type CheckinDistributionItem struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Color  string `json:"color"`
}

// ResponseTimePoint represents one account's API key test latency.
type ResponseTimePoint struct {
	AccountName string `json:"accountName"`
	SiteName    string `json:"siteName"`
	LatencyMs   int64  `json:"latencyMs"`
	Status      string `json:"status"`
}

// SiteReliability aggregates per-site checkin reliability over the analytics window.
type SiteReliability struct {
	SiteID        string  `json:"siteId"`
	SiteName      string  `json:"siteName"`
	TotalCheckins int     `json:"totalCheckins"`
	SuccessRate   float64 `json:"successRate"`
	AvgLatencyMs  int64   `json:"avgLatencyMs"`
	LastCheckinAt string  `json:"lastCheckinAt"`
}

// BalanceDeltaPoint represents one day's balance change and cumulative total.
type BalanceDeltaPoint struct {
	Date       string   `json:"date"`
	Delta      float64  `json:"delta"`
	Cumulative *float64 `json:"cumulative,omitempty"`
}

// AnalyticsResult aggregates all analytics data for the dashboard.
type AnalyticsResult struct {
	GeneratedAt         string                    `json:"generatedAt"`
	Days                int                       `json:"days"`
	BalanceTrend        []BalanceTrendPoint       `json:"balanceTrend"`
	CheckinDistribution []CheckinDistributionItem `json:"checkinDistribution"`
	ResponseTimes       []ResponseTimePoint       `json:"responseTimes"`
	SiteReliability     []SiteReliability         `json:"siteReliability"`
	BalanceDeltas       []BalanceDeltaPoint       `json:"balanceDeltas"`
}

// analyticsDaysBounds clamps the requested day window to a sane range.
func analyticsDaysBounds(raw string) int {
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 30
	}
	if days < 1 {
		return 1
	}
	if days > 365 {
		return 365
	}
	return days
}

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}

	ctx := r.Context()
	days := analyticsDaysBounds(r.URL.Query().Get("days"))
	result := AnalyticsResult{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Days:        days,
	}

	// Balance trend: daily average across all accounts over the selected window.
	// Use AVG(NULLIF(balance, 0)) so that NULL and zero balances are excluded
	// from the average rather than dragging it down.
	rows, err := a.db.QueryContext(ctx, `
		SELECT substr(created_at,1,10) as day, AVG(NULLIF(balance, 0))
		FROM balance_snapshots
		WHERE created_at >= datetime('now', ?)
		GROUP BY day
		ORDER BY day
	`, "-"+strconv.Itoa(days)+" days")
	if err != nil {
		log.Printf("[analytics] balance trend query failed: %v", err)
	} else {
		for rows.Next() {
			var day string
			var avg sql.NullFloat64
			if err := rows.Scan(&day, &avg); err != nil {
				log.Printf("[analytics] balance trend scan failed: %v", err)
				continue
			}
			if avg.Valid {
				val := avg.Float64
				result.BalanceTrend = append(result.BalanceTrend, BalanceTrendPoint{
					Date:    day,
					Balance: &val,
				})
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[analytics] balance trend iteration failed: %v", err)
		}
		rows.Close()
	}

	// Checkin distribution: last 7 days (independent of the selected window so the
	// status breakdown stays comparable across ranges).
	distRows, err := a.db.QueryContext(ctx, `
		SELECT status, COUNT(*) as cnt
		FROM checkin_logs
		WHERE started_at >= datetime('now', '-7 days')
		GROUP BY status
		ORDER BY cnt DESC
	`)
	if err != nil {
		log.Printf("[analytics] checkin distribution query failed: %v", err)
	} else {
		statusLabels := map[string]string{
			"success":         "成功",
			"already":         "已签到",
			"already_checked": "已签到",
			"failed":          "失败",
			"unsupported":     "不支持",
			"auth_expired":    "授权过期",
			"manual_required": "需手动",
		}
		statusColors := map[string]string{
			"success":         "#34b87a",
			"already":         "#4b8bf5",
			"already_checked": "#4b8bf5",
			"failed":          "#e85a6d",
			"unsupported":     "#8a9bb4",
			"auth_expired":    "#d4a03c",
			"manual_required": "#d4a03c",
		}
		for distRows.Next() {
			var status string
			var count int
			if err := distRows.Scan(&status, &count); err == nil {
				label := statusLabels[status]
				if label == "" {
					label = status
				}
				color := statusColors[status]
				if color == "" {
					color = "#8a9bb4"
				}
				result.CheckinDistribution = append(result.CheckinDistribution, CheckinDistributionItem{
					Status: status,
					Label:  label,
					Count:  count,
					Color:  color,
				})
			}
		}
		if err := distRows.Err(); err != nil {
			log.Printf("[analytics] checkin distribution iteration failed: %v", err)
		}
		distRows.Close()
	}

	// Response times: accounts with API key latency data.
	respRows, err := a.db.QueryContext(ctx, `
		SELECT a.display_name, s.name, COALESCE(a.api_key_latency_ms, 0), COALESCE(a.api_key_status, 'untested')
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.api_key_latency_ms > 0
		ORDER BY a.api_key_latency_ms DESC
		LIMIT 20
	`)
	if err != nil {
		log.Printf("[analytics] response times query failed: %v", err)
	} else {
		for respRows.Next() {
			var item ResponseTimePoint
			if err := respRows.Scan(&item.AccountName, &item.SiteName, &item.LatencyMs, &item.Status); err == nil {
				result.ResponseTimes = append(result.ResponseTimes, item)
			}
		}
		if err := respRows.Err(); err != nil {
			log.Printf("[analytics] response times iteration failed: %v", err)
		}
		respRows.Close()
	}

	// Site reliability: per-site checkin success rate over the selected window.
	siteRows, err := a.db.QueryContext(ctx, `
		SELECT s.id, s.name,
		       COUNT(l.id) AS total,
		       SUM(CASE WHEN l.status IN ('success','already_checked') THEN 1 ELSE 0 END) AS ok,
		       CAST(AVG((julianday(l.finished_at) - julianday(l.started_at)) * 86400000.0) AS INTEGER) AS avg_ms,
		       MAX(l.started_at) AS last_at
		FROM upstream_sites s
		LEFT JOIN checkin_logs l ON l.upstream_site_id = s.id
		    AND l.started_at >= datetime('now', ?)
		GROUP BY s.id, s.name
		HAVING total > 0
		ORDER BY ok * 1.0 / total DESC, total DESC
	`, "-"+strconv.Itoa(days)+" days")
	if err != nil {
		log.Printf("[analytics] site reliability query failed: %v", err)
	} else {
		for siteRows.Next() {
			var item SiteReliability
			var okCount int
			var avgMs sql.NullInt64
			var lastAt sql.NullString
			if err := siteRows.Scan(&item.SiteID, &item.SiteName, &item.TotalCheckins, &okCount, &avgMs, &lastAt); err == nil {
				if item.TotalCheckins > 0 {
					item.SuccessRate = float64(okCount) / float64(item.TotalCheckins)
				}
				if avgMs.Valid {
					item.AvgLatencyMs = avgMs.Int64
				}
				if lastAt.Valid {
					item.LastCheckinAt = lastAt.String
				}
				result.SiteReliability = append(result.SiteReliability, item)
			}
		}
		if err := siteRows.Err(); err != nil {
			log.Printf("[analytics] site reliability iteration failed: %v", err)
		}
		siteRows.Close()
	}

	// Balance deltas: daily change vs the previous day's average, plus cumulative total.
	result.BalanceDeltas = computeBalanceDeltas(result.BalanceTrend)

	writeJSON(w, http.StatusOK, result)
}

// computeBalanceDeltas turns a daily average balance trend into day-over-day
// deltas with a running cumulative value.
func computeBalanceDeltas(trend []BalanceTrendPoint) []BalanceDeltaPoint {
	if len(trend) == 0 {
		return nil
	}
	deltas := make([]BalanceDeltaPoint, 0, len(trend))
	var prev *float64
	for _, point := range trend {
		current := 0.0
		if point.Balance != nil {
			current = *point.Balance
		}
		// 第一个数据点没有前一天可对比，增量应为 0（而非当天余额本身）。
		delta := 0.0
		if prev != nil {
			delta = current - *prev
		}
		cum := current
		deltas = append(deltas, BalanceDeltaPoint{
			Date:       point.Date,
			Delta:      delta,
			Cumulative: &cum,
		})
		prev = &current
	}
	return deltas
}
