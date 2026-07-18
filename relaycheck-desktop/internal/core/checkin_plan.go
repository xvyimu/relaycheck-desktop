package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	checkinPreviewTTL         = 5 * time.Minute
	maxPendingCheckinPreviews = 64
)

var (
	errCheckinPreviewLimit       = errors.New("checkin preview account limit exceeded")
	errCheckinPreviewCapacity    = errors.New("checkin preview capacity reached")
	errCheckinPreviewUnavailable = errors.New("checkin preview unavailable")
)

type CheckinExecutionPlan struct {
	ID          string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Candidates  []DryRunPreviewItem
	RunAccounts []checkinRunAccount
}

type CheckinPlanStore struct {
	mu    sync.Mutex
	plans map[string]CheckinExecutionPlan
	now   func() time.Time
}

func NewCheckinPlanStore() *CheckinPlanStore {
	return &CheckinPlanStore{
		plans: map[string]CheckinExecutionPlan{},
		now:   time.Now,
	}
}

func (s *CheckinPlanStore) Put(plan CheckinExecutionPlan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.purgeExpiredLocked(s.now())
	if len(s.plans) >= maxPendingCheckinPreviews {
		return errCheckinPreviewCapacity
	}
	s.plans[plan.ID] = cloneCheckinExecutionPlan(plan)
	return nil
}

func (s *CheckinPlanStore) Claim(id string) (CheckinExecutionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nowTime := s.now()
	s.purgeExpiredLocked(nowTime)
	plan, ok := s.plans[strings.TrimSpace(id)]
	if !ok || !plan.ExpiresAt.After(nowTime) {
		return CheckinExecutionPlan{}, errCheckinPreviewUnavailable
	}
	delete(s.plans, plan.ID)
	return cloneCheckinExecutionPlan(plan), nil
}

func (s *CheckinPlanStore) purgeExpiredLocked(nowTime time.Time) {
	for id, plan := range s.plans {
		if !plan.ExpiresAt.After(nowTime) {
			delete(s.plans, id)
		}
	}
}

func (s *CheckinPlanStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeExpiredLocked(s.now())
	return len(s.plans)
}

func cloneCheckinExecutionPlan(plan CheckinExecutionPlan) CheckinExecutionPlan {
	plan.Candidates = append([]DryRunPreviewItem(nil), plan.Candidates...)
	plan.RunAccounts = append([]checkinRunAccount(nil), plan.RunAccounts...)
	return plan
}

type CheckinPlanService struct {
	checkinBatch *CheckinBatchOrchestrator
	accountAuth  *AccountAuthRepository
	store        *CheckinPlanStore
	now          func() time.Time
}

func NewCheckinPlanService(app *App) *CheckinPlanService {
	return &CheckinPlanService{
		checkinBatch: app.checkinBatch,
		accountAuth:  app.accountAuth,
		store:        app.checkinPlanStore,
		now:          time.Now,
	}
}

func (s *CheckinPlanService) BuildAllDue(ctx context.Context) (DryRunPreview, error) {
	accounts, err := s.checkinBatch.LoadDueAccounts(ctx, "", maxDryRunAccounts+1)
	if err != nil {
		return DryRunPreview{}, err
	}
	if len(accounts) > maxDryRunAccounts {
		return DryRunPreview{}, errCheckinPreviewLimit
	}

	accounts = dedupeCheckinRunAccounts(accounts)
	auths, err := s.accountAuth.LoadBatch(ctx, checkinRunAccountIDs(accounts))
	if err != nil {
		return DryRunPreview{}, err
	}

	createdAt := s.now().UTC()
	preview := DryRunPreview{
		Type:          string(TaskCheckin),
		MaxAccounts:   maxDryRunAccounts,
		TotalAccounts: len(accounts),
		Items:         make([]DryRunPreviewItem, 0, len(accounts)),
	}
	runAccounts := make([]checkinRunAccount, 0, len(accounts))
	for _, account := range accounts {
		item := DryRunPreviewItem{
			AccountID:   account.ID,
			AccountName: account.AccountName,
			SiteName:    account.SiteName,
		}
		auth, ok := auths[account.ID]
		switch {
		case !ok:
			item.Action = "skip_not_found"
			item.Reason = "账号在预览过程中已被删除"
			preview.Skipped++
		case !auth.SupportsCheckin:
			item.Action = "skip_unsupported"
			item.Reason = "站点未配置可用签到规则"
			preview.Skipped++
		case !canAttemptCheckin(auth):
			item.Action = "skip_missing_credentials"
			item.Reason = "缺少 Cookie、令牌、API Key 或登录密码"
			preview.Skipped++
		default:
			item.Action = "will_run"
			item.Reason = "本地认证条件已就绪，将尝试签到"
			preview.WillRun++
			runAccounts = append(runAccounts, account)
		}
		preview.Items = append(preview.Items, item)
	}

	if preview.WillRun == 0 {
		return preview, nil
	}
	plan := CheckinExecutionPlan{
		ID:          newID(),
		CreatedAt:   createdAt,
		ExpiresAt:   createdAt.Add(checkinPreviewTTL),
		Candidates:  preview.Items,
		RunAccounts: runAccounts,
	}
	if err := s.store.Put(plan); err != nil {
		return DryRunPreview{}, err
	}
	preview.PreviewID = plan.ID
	preview.ExpiresAt = plan.ExpiresAt.Format(time.RFC3339Nano)
	return preview, nil
}

func (s *CheckinPlanService) Claim(previewID string) (CheckinExecutionPlan, error) {
	return s.store.Claim(previewID)
}

func dedupeCheckinRunAccounts(accounts []checkinRunAccount) []checkinRunAccount {
	seen := make(map[string]struct{}, len(accounts))
	result := make([]checkinRunAccount, 0, len(accounts))
	for _, account := range accounts {
		id := strings.TrimSpace(account.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		account.ID = id
		result = append(result, account)
	}
	return result
}

func canAttemptCheckin(auth accountAuthContext) bool {
	if !auth.SupportsCheckin {
		return false
	}
	return strings.TrimSpace(auth.Cookie) != "" ||
		strings.TrimSpace(auth.AccessToken) != "" ||
		strings.TrimSpace(auth.APIKey) != "" ||
		(strings.TrimSpace(auth.LoginName) != "" && strings.TrimSpace(auth.Password) != "")
}
