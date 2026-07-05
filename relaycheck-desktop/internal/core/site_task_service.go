package core

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type SiteTaskService struct {
	db                      *sql.DB
	rootCtx                 context.Context
	taskRunner              *TaskRunner
	detectSite              func(context.Context, string, string, string) bulkDetectSiteResult
	syncChannelModels       func(context.Context, channelModelSyncRecord) channelModelSyncItem
	invalidateReadCacheKeys func(...string)
}

func NewSiteTaskService(app *App) *SiteTaskService {
	return &SiteTaskService{
		db:                      app.db,
		rootCtx:                 app.rootCtx,
		taskRunner:              app.taskRunner,
		detectSite:              app.detectAndSaveSite,
		syncChannelModels:       app.syncChannelModels,
		invalidateReadCacheKeys: app.invalidateReadCacheKeys,
	}
}

func (s *SiteTaskService) StartDetectSites(taskID string, params map[string]interface{}) {
	go func() {
		ctx := s.rootContext()
		limit := 50
		onlyUnknownOrOpenAI := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if u, ok := params["onlyUnknownOrOpenAI"].(bool); ok {
			onlyUnknownOrOpenAI = u
		}

		jobs, err := s.loadDetectSiteJobs(ctx, limit, onlyUnknownOrOpenAI)
		if err != nil {
			task, _ := s.taskRunner.start(taskID, TaskDetectSites, 0)
			task.finish(err)
			return
		}

		task, taskCtx := s.taskRunner.start(taskID, TaskDetectSites, len(jobs))
		for _, job := range jobs {
			if taskCtx.Err() != nil {
				task.finish(taskCtx.Err())
				return
			}
			result := s.detectSite(taskCtx, job.ID, job.Name, job.BaseURL)
			item := ItemResult{ID: job.ID, Name: job.Name, Status: "success"}
			if result.Kind != "" {
				item.Message = result.Kind
			}
			task.update(item)
		}
		task.finish(nil)
	}()
}

func (s *SiteTaskService) StartChannelHealthProbe(taskID string, params map[string]interface{}) {
	go func() {
		ctx := s.rootContext()
		limit := 20
		onlyRisky := false
		if l, ok := params["limit"].(float64); ok && l > 0 {
			limit = int(l)
		}
		if limit > 50 {
			limit = 50
		}
		if r, ok := params["onlyRisky"].(bool); ok {
			onlyRisky = r
		}
		jobs, err := s.loadChannelHealthProbeJobs(ctx, limit, onlyRisky)
		if err != nil {
			task, _ := s.taskRunner.start(taskID, TaskChannelHealthProbe, 0)
			task.finish(err)
			return
		}

		task, taskCtx := s.taskRunner.start(taskID, TaskChannelHealthProbe, len(jobs))
		_ = s.runChannelHealthProbe(taskCtx, jobs, func(item ItemResult) {
			task.update(item)
		})
		if taskCtx.Err() != nil {
			task.finish(taskCtx.Err())
			return
		}
		task.finish(nil)
	}()
}

func (s *SiteTaskService) RunScheduledChannelHealthProbe(ctx context.Context, config channelHealthScheduleConfig) (channelHealthProbeResult, error) {
	jobs, err := s.loadChannelHealthProbeJobs(ctx, config.Limit, config.OnlyRisky)
	if err != nil {
		return channelHealthProbeResult{}, err
	}
	return s.runChannelHealthProbe(ctx, jobs, nil), nil
}

type detectSiteTaskJob struct {
	ID      string
	Name    string
	BaseURL string
}

func (s *SiteTaskService) loadDetectSiteJobs(ctx context.Context, limit int, onlyUnknownOrOpenAI bool) ([]detectSiteTaskJob, error) {
	query := `
		SELECT id, name, base_url
		FROM upstream_sites
		WHERE COALESCE(base_url,'') <> ''
		  AND lower(name) NOT LIKE '%9router%'
		  AND lower(base_url) <> 'http://localhost:20128'
	`
	args := []interface{}{}
	if onlyUnknownOrOpenAI {
		query += ` AND kind IN ('unknown','openai_compatible')`
	}
	query += ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []detectSiteTaskJob{}
	for rows.Next() {
		var job detectSiteTaskJob
		if err := rows.Scan(&job.ID, &job.Name, &job.BaseURL); err != nil {
			log.Printf("[task:detect-sites] scan failed: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[task:detect-sites] query iteration failed: %v", err)
	}
	return jobs, nil
}

type channelHealthProbeJob struct {
	ID      string
	Name    string
	BaseURL string
}

type channelHealthProbeResult struct {
	Total     int
	Processed int
	Failed    int
	Warning   int
	Items     []ItemResult
	Messages  []string
}

func (r channelHealthProbeResult) Summary() string {
	parts := []string{
		fmt.Sprintf("processed %d/%d", r.Processed, r.Total),
		fmt.Sprintf("warnings %d", r.Warning),
		fmt.Sprintf("failed %d", r.Failed),
	}
	if len(r.Messages) > 0 {
		parts = append(parts, strings.Join(r.Messages, "; "))
	}
	samples := r.RiskSamples(3)
	if len(samples) > 0 {
		parts = append(parts, "samples: "+strings.Join(samples, "; "))
	}
	return strings.Join(parts, "; ")
}

func (r channelHealthProbeResult) RiskSamples(limit int) []string {
	if limit <= 0 {
		return nil
	}
	samples := []string{}
	for _, item := range r.Items {
		if item.Status != "warning" && item.Status != "failed" {
			continue
		}
		sample := strings.TrimSpace(item.Name)
		if sample == "" {
			sample = item.ID
		}
		if item.Message != "" {
			sample += " (" + item.Message + ")"
		}
		samples = append(samples, sample)
		if len(samples) >= limit {
			break
		}
	}
	return samples
}

func (s *SiteTaskService) loadChannelHealthProbeJobs(ctx context.Context, limit int, onlyRisky bool) ([]channelHealthProbeJob, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	query := `
		SELECT s.id, s.name, s.base_url
		FROM upstream_sites s
		WHERE COALESCE(s.base_url,'') <> ''
		  AND s.id <> ?
		  AND lower(s.name) NOT LIKE '%9router%'
		  AND lower(s.base_url) <> 'http://localhost:20128'
	`
	args := []interface{}{globalScheduleSiteID}
	if onlyRisky {
		query += ` AND (
			s.health_status IN ('unknown','unreachable','down','failed','error')
			OR EXISTS (
				SELECT 1 FROM imported_channels c
				WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
				  AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
				  AND (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
				  AND COALESCE(c.models_status,'unchecked') IN ('unchecked','failed','key_invalid','empty','')
			)
			OR EXISTS (
				SELECT 1 FROM channel_accounts account
				WHERE account.upstream_site_id = s.id
				  AND COALESCE(account.api_key_fingerprint,'') <> ''
				  AND COALESCE(account.api_key_status,'unchecked') NOT IN ('valid')
			)
		)`
	}
	query += ` ORDER BY s.updated_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []channelHealthProbeJob{}
	for rows.Next() {
		var job channelHealthProbeJob
		if err := rows.Scan(&job.ID, &job.Name, &job.BaseURL); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *SiteTaskService) runChannelHealthProbe(ctx context.Context, jobs []channelHealthProbeJob, onItem func(ItemResult)) channelHealthProbeResult {
	result := channelHealthProbeResult{Total: len(jobs), Items: []ItemResult{}}
	for _, job := range jobs {
		if ctx.Err() != nil {
			result.Messages = append(result.Messages, ctx.Err().Error())
			break
		}
		item := s.probeChannelHealthSite(ctx, job.ID, job.Name, job.BaseURL)
		result.Processed++
		switch item.Status {
		case "failed":
			result.Failed++
		case "warning":
			result.Warning++
		}
		result.Items = append(result.Items, item)
		if onItem != nil {
			onItem(item)
		}
	}
	// Per-key invalidation: channel health probe changes channel health data,
	// which affects channel-health-overview, channels-list, action-center
	// (channel health risks), and dashboard-summary (counts).
	s.invalidateReadCacheKeys("channel-health-overview", "channels-list", "action-center", "dashboard-summary")
	return result
}

func (s *SiteTaskService) probeChannelHealthSite(ctx context.Context, id, name, baseURL string) ItemResult {
	detection := s.detectSite(ctx, id, name, baseURL)
	item := ItemResult{ID: id, Name: name, Status: "success", Message: detection.HealthStatus}
	if detection.Error != "" {
		item.Status = "failed"
		item.Message = detection.Error
		return item
	}
	s.syncModelsForHealthSite(ctx, id, detection.BaseURL)
	switch strings.ToLower(detection.HealthStatus) {
	case "unreachable", "down", "failed", "error":
		item.Status = "failed"
	case "unknown", "degraded", "auth_required", "blocked":
		item.Status = "warning"
	}
	return item
}

func (s *SiteTaskService) syncModelsForHealthSite(ctx context.Context, siteID string, baseURL string) {
	records, err := s.loadChannelModelSyncRecordsForHealthSite(ctx, siteID, baseURL)
	if err != nil {
		return
	}
	for _, record := range records {
		if ctx.Err() != nil {
			return
		}
		s.syncChannelModels(ctx, record)
	}
}

func (s *SiteTaskService) loadChannelModelSyncRecordsForHealthSite(ctx context.Context, siteID string, baseURL string) ([]channelModelSyncRecord, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, COALESCE(c.base_url,''), c.upstream_kind, COALESCE(c.raw_json,''),
		       COALESCE(c.channel_key_encrypted,''), COALESCE(c.model_count,0),
		       COALESCE(c.sample_models_json,''), COALESCE(c.models_source,''), COALESCE(c.models_status,''),
		       COALESCE(c.models_last_synced_at,''), COALESCE(c.models_message,'')
		FROM imported_channels c
		LEFT JOIN upstream_sites s
		  ON (s.channel_id = c.id OR (COALESCE(s.channel_id,'') = '' AND COALESCE(s.base_url,'') <> '' AND s.base_url = COALESCE(c.base_url,'')))
		WHERE COALESCE(c.source_sync_status,'active') <> 'archived'
		  AND c.upstream_kind IN ('newapi','oneapi','sub2api','modified_relay')
		  AND (s.id = ? OR (? <> '' AND COALESCE(c.base_url,'') = ?))
		ORDER BY c.updated_at DESC
		LIMIT 10
	`, siteID, baseURL, baseURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []channelModelSyncRecord{}
	for rows.Next() {
		var record channelModelSyncRecord
		if err := rows.Scan(&record.ID, &record.Name, &record.BaseURL, &record.Kind, &record.RawJSON, &record.ChannelKeyEncrypted, &record.ModelCount, &record.SampleModelsJSON, &record.ModelsSource, &record.ModelsStatus, &record.ModelsLastSyncedAt, &record.ModelsMessage); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (s *SiteTaskService) rootContext() context.Context {
	if s.rootCtx != nil {
		return s.rootCtx
	}
	return context.Background()
}
