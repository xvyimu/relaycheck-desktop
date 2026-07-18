package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// maxDryRunAccounts limits the number of accounts in a single dry-run preview
// to prevent abuse and keep response times reasonable.
const maxDryRunAccounts = 200

// DryRunPreview shows what would happen if a batch operation is executed.
// POST /api/tasks/dry-run
// Checkin body: {"type":"checkin","scope":{"kind":"all_due"}}
// Test/identify compatibility body: {"type":"test|identify","accountIds":["id1"]}
type DryRunPreview struct {
	Type          string              `json:"type"`
	PreviewID     string              `json:"previewId,omitempty"`
	ExpiresAt     string              `json:"expiresAt,omitempty"`
	MaxAccounts   int                 `json:"maxAccounts"`
	TotalAccounts int                 `json:"totalAccounts"`
	WillRun       int                 `json:"willRun"`
	Skipped       int                 `json:"skipped"`
	Items         []DryRunPreviewItem `json:"items"`
}

type DryRunPreviewItem struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	SiteName    string `json:"siteName"`
	Action      string `json:"action"` // "will_run" | "skip_no_cookie" | "skip_expired" | "skip_unsupported" | "skip_already"
	Reason      string `json:"reason"`
}

func (a *App) handleDryRun(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}

	var body struct {
		Type       string   `json:"type"`
		AccountIDs []string `json:"accountIds"`
		Scope      struct {
			Kind string `json:"kind"`
		} `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败")
		return
	}
	body.Type = strings.TrimSpace(body.Type)
	if body.Type == "" {
		writeError(w, http.StatusBadRequest, "缺少任务类型")
		return
	}
	if body.Type == string(TaskCheckin) {
		if strings.TrimSpace(body.Scope.Kind) != "all_due" {
			writeError(w, http.StatusBadRequest, "签到预览需要 all_due 范围")
			return
		}
		preview, err := a.checkinPlans.BuildAllDue(r.Context())
		switch {
		case errors.Is(err, errCheckinPreviewLimit):
			writeError(w, http.StatusConflict, fmt.Sprintf("单次预览最多支持 %d 个账号，请先缩小待签到范围。", maxDryRunAccounts))
			return
		case errors.Is(err, errCheckinPreviewCapacity):
			writeError(w, http.StatusTooManyRequests, "待确认的签到预览过多，请稍后重试。")
			return
		case err != nil:
			writePublicError(w, http.StatusInternalServerError, "签到预览生成失败，请稍后重试。", err)
			return
		}
		writeJSON(w, http.StatusOK, preview)
		return
	}
	if len(body.AccountIDs) == 0 {
		writeError(w, http.StatusBadRequest, "缺少类型或账号 ID")
		return
	}
	if len(body.AccountIDs) > maxDryRunAccounts {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("单次预览最多支持 %d 个账号", maxDryRunAccounts))
		return
	}

	ctx := r.Context()
	preview := DryRunPreview{
		Type:          body.Type,
		MaxAccounts:   maxDryRunAccounts,
		TotalAccounts: len(body.AccountIDs),
		Items:         []DryRunPreviewItem{},
	}

	// Build a single batch query to avoid N+1.
	// Use placeholders for each account ID.
	placeholders := make([]string, len(body.AccountIDs))
	args := make([]interface{}, len(body.AccountIDs))
	for i, id := range body.AccountIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `
		SELECT a.id, a.display_name, COALESCE(s.name,''), a.login_status, a.auth_type,
		       COALESCE(s.supports_checkin, 0)
		FROM channel_accounts a
		JOIN upstream_sites s ON s.id = a.upstream_site_id
		WHERE a.id IN (` + strings.Join(placeholders, ",") + `)
	`
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	// Build a map of found accounts
	type accountInfo struct {
		AccountName     string
		SiteName        string
		LoginStatus     string
		AuthType        string
		SupportsCheckin int
	}
	found := make(map[string]accountInfo, len(body.AccountIDs))
	for rows.Next() {
		var id, accountName, siteName, loginStatus, authType string
		var supportsCheckin int
		if err := rows.Scan(&id, &accountName, &siteName, &loginStatus, &authType, &supportsCheckin); err != nil {
			continue
		}
		found[id] = accountInfo{
			AccountName:     accountName,
			SiteName:        siteName,
			LoginStatus:     loginStatus,
			AuthType:        authType,
			SupportsCheckin: supportsCheckin,
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[dry-run] account info iteration failed: %v", err)
	}
	rows.Close()

	// Iterate in the original order to produce preview items
	for _, accountID := range body.AccountIDs {
		var item DryRunPreviewItem
		item.AccountID = accountID

		info, ok := found[accountID]
		if !ok {
			item.AccountName = "未知"
			item.SiteName = "未知"
			item.Action = "skip_not_found"
			item.Reason = "账号不存在"
			preview.Skipped++
			preview.Items = append(preview.Items, item)
			continue
		}

		item.AccountName = info.AccountName
		item.SiteName = info.SiteName

		switch body.Type {
		case "test":
			item.Action = "will_run"
			item.Reason = "将测试 API Key"
			preview.WillRun++
		case "identify":
			item.Action = "will_run"
			item.Reason = "将识别渠道类型"
			preview.WillRun++
		default:
			item.Action = "skip_unknown_type"
			item.Reason = "未知操作类型"
			preview.Skipped++
		}

		preview.Items = append(preview.Items, item)
	}

	writeJSON(w, http.StatusOK, preview)
}
