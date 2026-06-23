package core

import (
	"net/http"
	"sort"
	"strings"
)

func (a *App) handleActionCenter(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	center, err := a.actionCenter(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, center)
}

func (a *App) actionCenter(r *http.Request) (ActionCenter, error) {
	items := []ActionItem{}
	type actionQuery struct {
		id          string
		priority    int
		level       string
		title       string
		description string
		target      string
		filter      string
		action      string
		countSQL    string
		sampleSQL   string
	}
	queries := []actionQuery{
		{
			id:          "auth-required-accounts",
			priority:    100,
			level:       "danger",
			title:       "优先处理失效授权",
			description: "这些账号登录态已过期或需要人工介入，会直接影响签到和余额刷新。",
			target:      "accounts",
			filter:      "problem",
			action:      "进入账号页，批量打开授权或逐个保存网页登录会话。",
			countSQL:    `SELECT COUNT(*) FROM channel_accounts WHERE login_status IN ('expired','manual_required','captcha_required','two_factor_required') OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required')`,
			sampleSQL:   `SELECT a.display_name || ' · ' || s.name FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE a.login_status IN ('expired','manual_required','captcha_required','two_factor_required') OR COALESCE(a.last_checkin_status,'') IN ('auth_expired','manual_required') ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:          "api-key-problems",
			priority:    92,
			level:       "danger",
			title:       "检测异常 API Key",
			description: "已保存的密钥存在无效或未检测状态，可能导致同站多账号识别和余额查询不准确。",
			target:      "accounts",
			filter:      "problem",
			action:      "在账号页点击“批量检测密钥”，或单个账号点击“检测密钥”。",
			countSQL:    `SELECT COUNT(*) FROM channel_accounts WHERE COALESCE(api_key_fingerprint,'') <> '' AND COALESCE(api_key_status,'unchecked') NOT IN ('valid')`,
			sampleSQL:   `SELECT a.display_name || ' · ' || COALESCE(a.api_key_status,'unchecked') FROM channel_accounts a WHERE COALESCE(a.api_key_fingerprint,'') <> '' AND COALESCE(a.api_key_status,'unchecked') NOT IN ('valid') ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:          "today-checkin-problems",
			priority:    84,
			level:       "warning",
			title:       "复查今日签到异常",
			description: "今日有账号签到失败、授权过期或站点不支持，需要分清是登录问题还是站点未开启签到。",
			target:      "checkins",
			filter:      "problem",
			action:      "进入签到页筛选异常记录，先处理 auth_expired，再确认 unsupported 是否为站点未开启。",
			countSQL:    `SELECT COUNT(*) FROM checkin_logs WHERE status NOT IN ('success','already_checked') AND substr(started_at,1,10)=substr(datetime('now'),1,10)`,
			sampleSQL:   `SELECT a.display_name || ' · ' || s.name || ' · ' || l.status FROM checkin_logs l JOIN channel_accounts a ON a.id=l.account_id JOIN upstream_sites s ON s.id=l.upstream_site_id WHERE l.status NOT IN ('success','already_checked') AND substr(l.started_at,1,10)=substr(datetime('now'),1,10) ORDER BY l.started_at DESC LIMIT 4`,
		},
		{
			id:          "balance-missing",
			priority:    72,
			level:       "warning",
			title:       "刷新缺失余额",
			description: "部分支持余额的站点账号还没有余额快照，低余额提醒和额度判断会不完整。",
			target:      "accounts",
			filter:      "all",
			action:      "进入账号页，对这些账号刷新余额；若失败，再检查授权或余额接口识别。",
			countSQL:    `SELECT COUNT(*) FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE s.supports_balance=1 AND a.balance IS NULL`,
			sampleSQL:   `SELECT a.display_name || ' · ' || s.name FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE s.supports_balance=1 AND a.balance IS NULL ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:          "low-balance",
			priority:    70,
			level:       "warning",
			title:       "关注低余额账号",
			description: "部分账号余额数值较低，建议优先检查是否还能正常使用。",
			target:      "balances",
			filter:      "",
			action:      "进入余额页查看最近快照，必要时刷新余额或更换账号。",
			countSQL:    `SELECT COUNT(*) FROM channel_accounts WHERE balance IS NOT NULL AND balance_unit IN ('quota','unknown','cny','usd') AND balance <= 5`,
			sampleSQL:   `SELECT display_name || ' · 余额 ' || ROUND(balance, 3) || ' ' || COALESCE(balance_unit,'unknown') FROM channel_accounts WHERE balance IS NOT NULL AND balance_unit IN ('quota','unknown','cny','usd') AND balance <= 5 ORDER BY balance ASC LIMIT 4`,
		},
		{
			id:          "unknown-channels",
			priority:    60,
			level:       "warning",
			title:       "识别未知渠道",
			description: "渠道后台类型未知会影响签到、余额、模型和价格能力判断。",
			target:      "channels",
			filter:      "unknown",
			action:      "进入渠道页筛选未知渠道，点击“识别并生成站点”。",
			countSQL:    `SELECT COUNT(*) FROM imported_channels WHERE upstream_kind='unknown' AND COALESCE(source_sync_status,'active') <> 'archived'`,
			sampleSQL:   `SELECT name || ' · ' || COALESCE(base_url,'未配置 Base URL') FROM imported_channels WHERE upstream_kind='unknown' AND COALESCE(source_sync_status,'active') <> 'archived' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:          "missing-channels",
			priority:    45,
			level:       "info",
			title:       "整理源端已移除渠道",
			description: "NewAPI 源端已经不再返回这些渠道，本地仍保留账号、日志和余额历史。",
			target:      "channels",
			filter:      "missing",
			action:      "进入渠道页筛选源端已移除，确认后归档保留。",
			countSQL:    `SELECT COUNT(*) FROM imported_channels WHERE COALESCE(source_sync_status,'active')='missing'`,
			sampleSQL:   `SELECT name || ' · ' || COALESCE(base_url,'未配置 Base URL') FROM imported_channels WHERE COALESCE(source_sync_status,'active')='missing' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:          "unreachable-sites",
			priority:    40,
			level:       "warning",
			title:       "检查不可达站点",
			description: "站点不可达会导致识别、签到、余额刷新都失败。",
			target:      "sites",
			filter:      "unreachable",
			action:      "进入上游站点页重新识别，或检查网络、域名、防火墙。",
			countSQL:    `SELECT COUNT(*) FROM upstream_sites WHERE health_status='unreachable'`,
			sampleSQL:   `SELECT name || ' · ' || base_url FROM upstream_sites WHERE health_status='unreachable' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:          "unread-notifications",
			priority:    20,
			level:       "info",
			title:       "清理未读通知",
			description: "未读通知过多会掩盖新的失败、授权失效和低余额提醒。",
			target:      "notifications",
			filter:      "unread",
			action:      "进入通知中心处理关键提醒；确认无须处理后可一键已读。",
			countSQL:    `SELECT COUNT(*) FROM app_notifications WHERE read=0`,
			sampleSQL:   `SELECT title || ' · ' || content FROM app_notifications WHERE read=0 ORDER BY created_at DESC LIMIT 4`,
		},
	}

	for _, query := range queries {
		count, err := a.queryActionCount(r, query.countSQL)
		if err != nil {
			return ActionCenter{}, err
		}
		if count <= 0 {
			continue
		}
		samples, err := a.queryActionSamples(r, query.sampleSQL)
		if err != nil {
			return ActionCenter{}, err
		}
		items = append(items, ActionItem{
			ID:          query.id,
			Priority:    query.priority,
			Level:       query.level,
			Title:       query.title,
			Description: query.description,
			Count:       count,
			Target:      query.target,
			Filter:      query.filter,
			Action:      query.action,
			Samples:     samples,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority > items[j].Priority
	})

	return ActionCenter{
		GeneratedAt: now(),
		Overall:     actionCenterLevel(items),
		Items:       items,
	}, nil
}

func (a *App) queryActionCount(r *http.Request, query string) (int, error) {
	var count int
	err := a.db.QueryRowContext(r.Context(), query).Scan(&count)
	return count, err
}

func (a *App) queryActionSamples(r *http.Request, query string) ([]string, error) {
	rows, err := a.db.QueryContext(r.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	samples := []string{}
	for rows.Next() {
		var sample string
		if err := rows.Scan(&sample); err != nil {
			return nil, err
		}
		if strings.TrimSpace(sample) != "" {
			samples = append(samples, sample)
		}
	}
	return samples, nil
}

func actionCenterLevel(items []ActionItem) string {
	overall := "success"
	for _, item := range items {
		if item.Level == "danger" {
			return "danger"
		}
		if item.Level == "warning" {
			overall = "warning"
		}
		if item.Level == "info" && overall == "success" {
			overall = "info"
		}
	}
	return overall
}
