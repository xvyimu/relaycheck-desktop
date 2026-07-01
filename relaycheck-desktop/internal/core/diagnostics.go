package core

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func (a *App) handleSystemDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	diagnostics, err := a.systemDiagnostics(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diagnostics)
}

func (a *App) systemDiagnostics(r *http.Request) (SystemDiagnostics, error) {
	items := []DiagnosticItem{}

	dbPath := filepath.Join(a.dataDir, "relaycheck.db")
	if info, err := os.Stat(dbPath); err == nil {
		items = append(items, DiagnosticItem{
			ID:          "database",
			Level:       "success",
			Title:       "SQLite 数据库可用",
			Description: fmt.Sprintf("数据库文件 %.1f MB", float64(info.Size())/1024/1024),
			Action:      "无需处理。建议定期在设置页备份数据库。",
		})
	} else {
		items = append(items, DiagnosticItem{
			ID:          "database",
			Level:       "danger",
			Title:       "SQLite 数据库不可访问",
			Description: "数据库文件不可访问，请检查 data 目录权限后重启工具。",
			Action:      "去设置页恢复最近备份；若没有备份，检查 data 目录权限后重启工具。",
			SolutionSteps: []string{
				"关闭正在占用数据库的旧 relaycheck 进程，确认 data 目录可读写。",
				"到设置页查看备份列表，先备份当前文件，再恢复最近可用的 relaycheck.db。",
				"恢复后重新打开工具并刷新系统自检；仍失败时保留错误信息再排查磁盘权限。",
			},
		})
	}

	if proxyConfig, err := a.loadNetworkProxyConfig(r.Context()); err != nil {
		items = append(items, DiagnosticItem{
			ID:          "network-proxy",
			Level:       "warning",
			Title:       "代理设置无法读取",
			Description: err.Error(),
			Action:      "去设置页检查 network.proxy JSON，保存后重新运行自检。",
			SolutionSteps: []string{
				"打开设置页，找到代理设置卡片或 system settings JSON 中的 network.proxy。",
				"使用类似 {\"enabled\":true,\"url\":\"http://127.0.0.1:7897\",\"bypassLocal\":true} 的格式。",
				"保存后点击“测试代理”，确认代理服务正在运行。",
			},
		})
	} else if proxyConfig.Enabled {
		items = append(items, DiagnosticItem{
			ID:          "network-proxy",
			Level:       "success",
			Title:       "全局代理已启用",
			Description: "外部站点探测、签到、余额和 API Key 检测会使用代理：" + maskProxyURL(proxyConfig.URL),
			Action:      "无需处理。若本地 NewAPI 无法访问，确认已开启绕过本地地址。",
		})
	} else {
		items = append(items, DiagnosticItem{
			ID:          "network-proxy",
			Level:       "success",
			Title:       "全局代理未启用",
			Description: "当前外部站点请求默认直连；需要访问受网络影响的站点时可在设置页开启代理。",
			Action:      "无需处理。遇到直连超时的中转站时，到设置页开启 127.0.0.1:7897 等本机代理。",
		})
	}

	counts := map[string]int{}
	queries := map[string]string{
		"localInstances":      `SELECT COUNT(*) FROM local_newapi_instances`,
		"channels":            `SELECT COUNT(*) FROM imported_channels`,
		"missingChannels":     `SELECT COUNT(*) FROM imported_channels WHERE COALESCE(source_sync_status,'active')='missing'`,
		"archivedChannels":    `SELECT COUNT(*) FROM imported_channels WHERE COALESCE(source_sync_status,'active')='archived'`,
		"unknownChannels":     `SELECT COUNT(*) FROM imported_channels WHERE upstream_kind='unknown'`,
		"unreachableSites":    `SELECT COUNT(*) FROM upstream_sites WHERE health_status='unreachable'`,
		"accounts":            `SELECT COUNT(*) FROM channel_accounts`,
		"invalidAccounts":     `SELECT COUNT(*) FROM channel_accounts WHERE login_status IN ('expired','manual_required','captcha_required','two_factor_required')`,
		"failedCheckinsToday": `SELECT COUNT(*) FROM checkin_logs WHERE status NOT IN ('success','already_checked') AND substr(started_at,1,10)=substr(datetime('now','+8 hours'),1,10)`,
		"unreadNotifications": `SELECT COUNT(*) FROM app_notifications WHERE read=0`,
		"cookieExpiringSoon":  `SELECT COUNT(*) FROM channel_accounts WHERE cookie_expiry_at != '' AND cookie_expiry_at != '' AND datetime(cookie_expiry_at) BETWEEN datetime('now') AND datetime('now','+7 days')`,
	}
	for key, query := range queries {
		var count int
		if err := a.db.QueryRowContext(r.Context(), query).Scan(&count); err != nil {
			return SystemDiagnostics{}, err
		}
		counts[key] = count
	}

	items = append(items, countBasedDiagnostic("local-instances", counts["localInstances"] > 0, "已记录本地 NewAPI 实例", "尚未记录 NewAPI 实例", counts["localInstances"], "去本机扫描页添加或导入 NewAPI 后台。", []string{
		"打开本机扫描页，先扫 3000/3001/8080/9999/3010 等常见端口。",
		"如果 NewAPI 不在常见端口，手动填写后台地址后保存。",
		"有系统访问令牌时优先用后台 API 导入；有数据库路径时用 SQLite 只读导入。",
	}))
	items = append(items, countBasedDiagnostic("channels", counts["channels"] > 0, "已导入渠道", "还没有导入渠道", counts["channels"], "从 NewAPI 后台 API 或 SQLite 导入 channels。", []string{
		"进入本机扫描页，选择已发现的 NewAPI 实例。",
		"点击一键同步，或使用后台 API / SQLite 导入 channels。",
		"导入后到渠道页筛选待识别渠道，执行识别并生成站点。",
	}))
	items = append(items, countBasedDiagnostic("accounts", counts["accounts"] > 0, "已添加渠道账号", "还没有渠道账号", counts["accounts"], "在账号页添加邮箱密码、API Key 或浏览器授权会话。", []string{
		"进入账号页，点击添加账号。",
		"同一站点多个账号要分别建卡，优先填写邮箱/用户名；有 API Key 时填入 Key 便于区分。",
		"需要网页登录的站点，点击网页登录一次并保存授权。",
	}))

	items = append(items, thresholdDiagnostic("missing-channels", counts["missingChannels"], "warning", "存在源端已移除渠道", "所有渠道都仍在源端返回", "去渠道页筛选源端已移除；确认不用后归档，误判则先重新同步。", []string{
		"进入渠道页，筛选“源端已移除”。",
		"如果 NewAPI 后台确实删除了该渠道，点击归档保留本地历史。",
		"如果是同步令牌或网络导致误判，先回本机扫描页一键同步对应实例。",
	}))
	items = append(items, thresholdDiagnostic("archived-channels", counts["archivedChannels"], "info", "存在已归档渠道", "暂无归档渠道", "去渠道页查看已归档；需要继续使用时恢复为活跃。", []string{
		"进入渠道页，把筛选切到“已归档”或“全部含归档”。",
		"确认仍要管理的渠道，恢复为活跃。",
		"确认长期不用的渠道保持归档即可，不会删除账号、余额或签到日志。",
	}))
	items = append(items, thresholdDiagnostic("unknown-channels", counts["unknownChannels"], "warning", "存在待识别渠道", "渠道后台类型均已识别", "去渠道页执行识别；无法自动识别时在账号/站点编辑里手动指定后台类型。", []string{
		"进入渠道页，筛选后台类型“待识别”。",
		"逐个点击识别并生成站点，或到上游站点页批量识别。",
		"如果是魔改 NewAPI/Sub2API，编辑账号或站点时手动指定为魔改中转后再刷新自检。",
	}))
	items = append(items, thresholdDiagnostic("unreachable-sites", counts["unreachableSites"], "warning", "存在不可达站点", "未发现不可达站点", "去上游站点页重识别；必要时确认代理、域名和登录页地址。", []string{
		"进入上游站点页，筛选或查看健康状态为不可达的站点。",
		"确认网址能在 Chrome 打开；如果需要代理，到设置页开启全局代理并测试 127.0.0.1:7897 是否可用。",
		"如果站点是魔改 NewAPI 但直连超时，手动指定为魔改中转，并保留登录页地址用于网页登录授权。",
	}))
	items = append(items, thresholdDiagnostic("invalid-accounts", counts["invalidAccounts"], "danger", "存在需要处理的账号登录态", "账号登录态未发现明显问题", "去账号页处理问题账号：网页登录、保存授权、测试登录态或检测密钥。", []string{
		"进入账号页，筛选“需要处理”。",
		"网页登录类账号：点击网页登录，人工登录完成后点击保存授权，再测试登录态。",
		"API Key 类账号：点击检测密钥，确认 Key 有效、模型可读取且最小模型调用可用。",
		"账号密码类账号：更新密码后执行测试登录态；遇到验证码或二次验证时改用网页登录授权。",
	}))
	items = append(items, thresholdDiagnostic("failed-checkins-today", counts["failedCheckinsToday"], "warning", "今日存在签到异常", "今日签到未发现失败记录", "打开签到页查看失败原因；按 unsupported/auth_expired/failed 分别处理。", []string{
		"进入签到页，筛选“需要处理”。",
		"auth_expired/manual_required：回账号页重新授权。",
		"unsupported：该站点可能未开启签到，保留标注即可；若你确认有签到接口，再补自定义签到规则。",
		"failed：查看返回消息，先测试登录态，再单账号重试签到。",
	}))
	items = append(items, thresholdDiagnostic("unread-notifications", counts["unreadNotifications"], "info", "存在未读通知", "通知中心没有未读消息", "去通知中心按类型查看，处理完成后一键已读。", []string{
		"进入通知中心，优先查看失败、授权失效、低余额类通知。",
		"根据通知跳转到账号、站点或签到页处理对应问题。",
		"处理完成后点击一键已读，避免旧通知继续干扰自检判断。",
	}))

	items = append(items, thresholdDiagnostic("cookie-expiring", counts["cookieExpiringSoon"], "warning", "存在 Cookie 临近过期账号", "Cookie 未临近过期", "去账号页重新登录或刷新授权，避免签到因 Cookie 过期失败。", []string{
		"在账号页筛选 Cookie 临近过期的账号。",
		"点击重新登录或保存新的浏览器授权会话。",
		"刷新后确认 cookie_expiry_at 已更新到较远的未来时间。",
	}))

	return SystemDiagnostics{
		GeneratedAt: now(),
		Overall:     overallDiagnosticLevel(items),
		Items:       items,
	}, nil
}

func countBasedDiagnostic(id string, ok bool, okTitle string, badTitle string, count int, action string, solutionSteps []string) DiagnosticItem {
	if ok {
		return DiagnosticItem{ID: id, Level: "success", Title: okTitle, Description: fmt.Sprintf("当前数量：%d", count), Action: "无需处理。", Count: count}
	}
	return DiagnosticItem{ID: id, Level: "warning", Title: badTitle, Description: "当前数量：0", Action: action, SolutionSteps: solutionSteps}
}

func thresholdDiagnostic(id string, count int, issueLevel string, issueTitle string, okTitle string, action string, solutionSteps []string) DiagnosticItem {
	if count > 0 {
		return DiagnosticItem{ID: id, Level: issueLevel, Title: issueTitle, Description: fmt.Sprintf("当前数量：%d", count), Action: action, SolutionSteps: solutionSteps, Count: count}
	}
	return DiagnosticItem{ID: id, Level: "success", Title: okTitle, Description: "当前数量：0", Action: "无需处理。"}
}

func overallDiagnosticLevel(items []DiagnosticItem) string {
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
