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
	center, err := cachedRead(a, "action-center", overviewReadCacheTTL, func() (ActionCenter, error) {
		return a.buildActionCenter(r)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, center)
}

func (a *App) buildActionCenter(r *http.Request) (ActionCenter, error) {
	items := []ActionItem{}
	type actionQuery struct {
		id                string
		priority          int
		level             string
		category          string
		title             string
		description       string
		impact            string
		target            string
		filter            string
		action            string
		recommendedAction string
		countSQL          string
		sampleSQL         string
		args              []interface{}
	}
	queries := []actionQuery{
		{
			id:                "auth-required-accounts",
			priority:          100,
			level:             "danger",
			category:          "auth",
			title:             "优先处理失效授权",
			description:       "这些账号登录态已过期或需要人工介入，会直接影响签到和余额刷新。",
			impact:            "签到、余额刷新、模型探活都会跳过或失败。",
			target:            "accounts",
			filter:            "problem",
			action:            "进入账号页，批量打开授权或逐个保存网页登录会话。",
			recommendedAction: "先处理需要人工登录的账号，再重新跑一次签到和余额刷新。",
			countSQL:          `SELECT COUNT(*) FROM channel_accounts WHERE login_status IN ('expired','manual_required','captcha_required','two_factor_required') OR COALESCE(last_checkin_status,'') IN ('auth_expired','manual_required')`,
			sampleSQL:         `SELECT a.display_name || ' · ' || s.name FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE a.login_status IN ('expired','manual_required','captcha_required','two_factor_required') OR COALESCE(a.last_checkin_status,'') IN ('auth_expired','manual_required') ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:                "api-key-problems",
			priority:          92,
			level:             "danger",
			category:          "key",
			title:             "检测异常 API Key",
			description:       "已保存的密钥存在无效或未检测状态，可能导致同站多账号识别和余额查询不准确。",
			impact:            "模型可用性、Key 导出和后续渠道健康判断会失真。",
			target:            "accounts",
			filter:            "problem",
			action:            "在账号页点击“批量检测密钥”，或单个账号点击“检测密钥”。",
			recommendedAction: "批量检测 Key；无效 Key 需要替换或从可用池移除。",
			countSQL:          `SELECT COUNT(*) FROM channel_accounts WHERE COALESCE(api_key_fingerprint,'') <> '' AND COALESCE(api_key_status,'unchecked') NOT IN ('valid')`,
			sampleSQL:         `SELECT a.display_name || ' · ' || COALESCE(a.api_key_status,'unchecked') FROM channel_accounts a WHERE COALESCE(a.api_key_fingerprint,'') <> '' AND COALESCE(a.api_key_status,'unchecked') NOT IN ('valid') ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:                "today-checkin-problems",
			priority:          84,
			level:             "warning",
			category:          "checkin",
			title:             "复查今日签到异常",
			description:       "今日有账号签到失败、授权过期或站点不支持，需要分清是登录问题还是站点未开启签到。",
			impact:            "当日奖励、余额变化和账号活跃度统计可能缺失。",
			target:            "checkins",
			filter:            "problem",
			action:            "进入签到页筛选异常记录，先处理 auth_expired，再确认 unsupported 是否为站点未开启。",
			recommendedAction: "按状态分组复查失败记录，授权问题修复后重新签到。",
			countSQL:          `SELECT COUNT(*) FROM checkin_logs WHERE status NOT IN ('success','already_checked') AND substr(started_at,1,10)=substr(datetime('now','+8 hours'),1,10)`,
			sampleSQL:         `SELECT a.display_name || ' · ' || s.name || ' · ' || l.status FROM checkin_logs l JOIN channel_accounts a ON a.id=l.account_id JOIN upstream_sites s ON s.id=l.upstream_site_id WHERE l.status NOT IN ('success','already_checked') AND substr(l.started_at,1,10)=substr(datetime('now','+8 hours'),1,10) ORDER BY l.started_at DESC LIMIT 4`,
		},
		{
			id:                "cookie-expiring",
			priority:          88,
			level:             "warning",
			category:          "auth",
			title:             "Cookie 临近过期",
			description:       "部分账号的 Cookie 预计在 7 天内过期，可能导致签到和余额刷新失败。",
			impact:            "自动任务可能在未来一周内开始连续失败。",
			target:            "accounts",
			filter:            "problem",
			action:            "进入账号页重新登录或刷新授权，更新 Cookie 有效期。",
			recommendedAction: "优先刷新即将过期的高价值账号登录态。",
			countSQL:          `SELECT COUNT(*) FROM channel_accounts WHERE cookie_expiry_at != '' AND datetime(cookie_expiry_at) BETWEEN datetime('now') AND datetime('now','+7 days')`,
			sampleSQL:         `SELECT a.display_name || ' · 过期于 ' || substr(a.cookie_expiry_at,1,10) FROM channel_accounts a WHERE a.cookie_expiry_at != '' AND datetime(a.cookie_expiry_at) BETWEEN datetime('now') AND datetime('now','+7 days') ORDER BY a.cookie_expiry_at ASC LIMIT 4`,
		},
		{
			id:                "balance-missing",
			priority:          72,
			level:             "warning",
			category:          "balance",
			title:             "刷新缺失余额",
			description:       "部分支持余额的站点账号还没有余额快照，低余额提醒和额度判断会不完整。",
			impact:            "无法判断低余额和消耗趋势，成本雷达会缺数据。",
			target:            "accounts",
			filter:            "all",
			action:            "进入账号页，对这些账号刷新余额；若失败，再检查授权或余额接口识别。",
			recommendedAction: "批量刷新余额；失败账号转入授权或站点能力排查。",
			countSQL:          `SELECT COUNT(*) FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE s.supports_balance=1 AND a.balance IS NULL`,
			sampleSQL:         `SELECT a.display_name || ' · ' || s.name FROM channel_accounts a JOIN upstream_sites s ON s.id=a.upstream_site_id WHERE s.supports_balance=1 AND a.balance IS NULL ORDER BY a.updated_at DESC LIMIT 4`,
		},
		{
			id:                "low-balance",
			priority:          70,
			level:             "warning",
			category:          "balance",
			title:             "关注低余额账号",
			description:       "部分账号余额数值较低，建议优先检查是否还能正常使用。",
			impact:            "低余额账号可能在调度任务或模型调用中突然不可用。",
			target:            "balances",
			filter:            "",
			action:            "进入余额页查看最近快照，必要时刷新余额或更换账号。",
			recommendedAction: "补充余额、替换账号，或降低这些账号的使用优先级。",
			countSQL:          `SELECT COUNT(*) FROM channel_accounts WHERE balance IS NOT NULL AND balance_unit IN ('quota','unknown','cny','usd') AND balance <= 5`,
			sampleSQL:         `SELECT display_name || ' · 余额 ' || ROUND(balance, 3) || ' ' || COALESCE(balance_unit,'unknown') FROM channel_accounts WHERE balance IS NOT NULL AND balance_unit IN ('quota','unknown','cny','usd') AND balance <= 5 ORDER BY balance ASC LIMIT 4`,
		},
		{
			id:                "unknown-channels",
			priority:          60,
			level:             "warning",
			category:          "channel",
			title:             "识别未知渠道",
			description:       "渠道后台类型未知会影响签到、余额、模型和价格能力判断。",
			impact:            "未知渠道无法进入完整账号运营和健康监控闭环。",
			target:            "channels",
			filter:            "unknown",
			action:            "进入渠道页筛选未知渠道，点击“识别并生成站点”。",
			recommendedAction: "先批量识别渠道，再为可用站点补齐账号与 Key。",
			countSQL:          `SELECT COUNT(*) FROM imported_channels WHERE upstream_kind='unknown' AND COALESCE(source_sync_status,'active') <> 'archived'`,
			sampleSQL:         `SELECT name || ' · ' || COALESCE(base_url,'未配置 Base URL') FROM imported_channels WHERE upstream_kind='unknown' AND COALESCE(source_sync_status,'active') <> 'archived' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:                "channel-health-risks",
			priority:          58,
			level:             "warning",
			category:          "health",
			title:             "复核渠道健康风险",
			description:       "渠道健康监控发现站点不可达、模型同步失败或 Key 状态异常，需要进入渠道页集中处理。",
			impact:            "模型可用性、自动任务和后续路由选择都会受到影响。",
			target:            "channels",
			filter:            "health",
			action:            "进入渠道页查看健康监控，优先处理不可达站点和模型异常渠道。",
			recommendedAction: "先刷新健康概览；不可达站点重跑探测，模型异常渠道同步模型并检查 Key 权限。",
			countSQL: `SELECT COUNT(*) FROM upstream_sites s
				WHERE s.id <> ?
				  AND (
				    s.health_status IN ('unreachable','down','failed','error')
				    OR EXISTS (
				      SELECT 1 FROM imported_channels c
				      WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
				        AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
				        AND (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
				        AND COALESCE(c.models_status,'unchecked') IN ('failed','key_invalid','empty')
				    )
				    OR EXISTS (
				      SELECT 1 FROM channel_accounts a
				      WHERE a.upstream_site_id = s.id
				        AND COALESCE(a.api_key_fingerprint,'') <> ''
				        AND COALESCE(a.api_key_status,'unchecked') NOT IN ('valid','unchecked','untested','')
				    )
				  )`,
		sampleSQL: `SELECT s.name || ' · ' || s.health_status FROM upstream_sites s
				WHERE s.id <> ?
				  AND (
				    s.health_status IN ('unreachable','down','failed','error')
				    OR EXISTS (
				      SELECT 1 FROM imported_channels c
				      WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
				        AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
				        AND (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
				        AND COALESCE(c.models_status,'unchecked') IN ('failed','key_invalid','empty')
				    )
				    OR EXISTS (
				      SELECT 1 FROM channel_accounts a
				      WHERE a.upstream_site_id = s.id
				        AND COALESCE(a.api_key_fingerprint,'') <> ''
				        AND COALESCE(a.api_key_status,'unchecked') NOT IN ('valid','unchecked','untested','')
				    )
				  )
				ORDER BY s.updated_at DESC LIMIT 4`,
		args: []interface{}{globalScheduleSiteID},
		},
		{
			id:                "missing-channels",
			priority:          45,
			level:             "info",
			category:          "channel",
			title:             "整理源端已移除渠道",
			description:       "NewAPI 源端已经不再返回这些渠道，本地仍保留账号、日志和余额历史。",
			impact:            "历史账号仍保留，但源端状态已经不适合继续当作活跃渠道。",
			target:            "channels",
			filter:            "missing",
			action:            "进入渠道页筛选源端已移除，确认后归档保留。",
			recommendedAction: "确认是否归档，避免已移除渠道干扰日常运营视图。",
			countSQL:          `SELECT COUNT(*) FROM imported_channels WHERE COALESCE(source_sync_status,'active')='missing'`,
			sampleSQL:         `SELECT name || ' · ' || COALESCE(base_url,'未配置 Base URL') FROM imported_channels WHERE COALESCE(source_sync_status,'active')='missing' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:                "unreachable-sites",
			priority:          40,
			level:             "warning",
			category:          "site",
			title:             "检查不可达站点",
			description:       "站点不可达会导致识别、签到、余额刷新都失败。",
			impact:            "该站点下账号、Key、余额和签到都会处于不可验证状态。",
			target:            "sites",
			filter:            "unreachable",
			action:            "进入上游站点页重新识别，或检查网络、域名、防火墙。",
			recommendedAction: "先重跑站点探测；持续不可达时暂停相关账号的自动任务。",
			countSQL:          `SELECT COUNT(*) FROM upstream_sites WHERE health_status='unreachable'`,
			sampleSQL:         `SELECT name || ' · ' || base_url FROM upstream_sites WHERE health_status='unreachable' ORDER BY updated_at DESC LIMIT 4`,
		},
		{
			id:                "unread-notifications",
			priority:          20,
			level:             "info",
			category:          "notification",
			title:             "清理未读通知",
			description:       "未读通知过多会掩盖新的失败、授权失效和低余额提醒。",
			impact:            "新的关键告警不容易被发现。",
			target:            "notifications",
			filter:            "unread",
			action:            "进入通知中心处理关键提醒；确认无须处理后可一键已读。",
			recommendedAction: "先处理 warning/danger 通知，再批量标记无须处理的历史消息。",
			countSQL:          `SELECT COUNT(*) FROM app_notifications WHERE read=0`,
			sampleSQL:         `SELECT title || ' · ' || content FROM app_notifications WHERE read=0 ORDER BY created_at DESC LIMIT 4`,
		},
	}

	for _, query := range queries {
		count, err := a.queryActionCount(r, query.countSQL, query.args)
		if err != nil {
			return ActionCenter{}, err
		}
		if count <= 0 {
			continue
		}
		samples, err := a.queryActionSamples(r, query.sampleSQL, query.args)
		if err != nil {
			return ActionCenter{}, err
		}
		items = append(items, ActionItem{
			ID:                query.id,
			Priority:          query.priority,
			Level:             query.level,
			Category:          query.category,
			Title:             query.title,
			Description:       query.description,
			Impact:            query.impact,
			Count:             count,
			Target:            query.target,
			Filter:            query.filter,
			Action:            query.action,
			RecommendedAction: query.recommendedAction,
			Samples:           samples,
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

func (a *App) queryActionCount(r *http.Request, query string, args []interface{}) (int, error) {
	var count int
	err := a.db.QueryRowContext(r.Context(), query, args...).Scan(&count)
	return count, err
}

func (a *App) queryActionSamples(r *http.Request, query string, args []interface{}) ([]string, error) {
	rows, err := a.db.QueryContext(r.Context(), query, args...)
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
	// Surface iteration errors (context cancellation, decode failures) that
	// would otherwise be silently dropped.
	if err := rows.Err(); err != nil {
		return nil, err
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
