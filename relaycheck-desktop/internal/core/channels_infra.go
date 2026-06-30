package core

import (
	"context"

	"relaycheck-desktop/internal/channels"
)

// This file hosts the exported adapter methods that *App implements to
// satisfy channels.Infra. The channels service calls these methods; they
// forward to the host's internal helpers and convert the host's private
// types (UpstreamDetection, schedulerRunRecord, checkinScheduleConfig) into
// the channels package's mirror types.
//
// Methods already provided by *App for other packages (DB, DoHTTP,
// DoHTTPWithTimeout, Notify, Audit, Now, NewID) are reused unchanged — see
// app.go, infra.go, and network.go.

// DecryptText is the exported adapter for the channels package's Infra
// interface. It delegates to the host's crypto service so the channels
// service can recover channel API keys without importing core.
func (a *App) DecryptText(ciphertext string) (string, error) {
	return a.crypto.Decrypt(ciphertext)
}

// DetectUpstream is the exported adapter for the channels package's Infra
// interface. It forwards to the host's sites-backed detectUpstream helper
// and converts the core.UpstreamDetection to channels.Detection so the
// channels service stays decoupled from core.
func (a *App) DetectUpstream(ctx context.Context, raw string) (channels.Detection, error) {
	d := a.detectUpstream(ctx, raw)
	return channels.Detection{
		BaseURL:             d.BaseURL,
		HomepageURL:         d.HomepageURL,
		LoginURL:            d.LoginURL,
		Kind:                d.Kind,
		HealthStatus:        d.HealthStatus,
		DetectionConfidence: d.DetectionConfidence,
		SupportsCheckin:     d.SupportsCheckin,
		SupportsBalance:     d.SupportsBalance,
		SupportsModels:      d.SupportsModels,
		SupportsPricing:     d.SupportsPricing,
		MatchedSignals:      d.MatchedSignals,
	}, nil
}

// EnsureUpstreamSiteForChannel is the exported adapter for the channels
// package's Infra interface. It forwards to the host's sites-backed
// ensureUpstreamSiteForChannel helper, converting the channels.EnsureSiteInput
// into the (channelID, name, rawBaseURL, kind, *UpstreamDetection) tuple the
// host helper expects.
func (a *App) EnsureUpstreamSiteForChannel(ctx context.Context, input channels.EnsureSiteInput) (string, bool, error) {
	var detection *UpstreamDetection
	if input.Detection != nil {
		detection = &UpstreamDetection{
			BaseURL:             input.Detection.BaseURL,
			HomepageURL:         input.Detection.HomepageURL,
			LoginURL:            input.Detection.LoginURL,
			Kind:                input.Detection.Kind,
			HealthStatus:        input.Detection.HealthStatus,
			DetectionConfidence: input.Detection.DetectionConfidence,
			SupportsCheckin:     input.Detection.SupportsCheckin,
			SupportsBalance:     input.Detection.SupportsBalance,
			SupportsModels:      input.Detection.SupportsModels,
			SupportsPricing:     input.Detection.SupportsPricing,
			MatchedSignals:      input.Detection.MatchedSignals,
		}
	}
	return a.ensureUpstreamSiteForChannel(ctx, input.ChannelID, input.Name, input.RawBaseURL, input.Kind, detection)
}

// InvalidateReadCache is the exported adapter for the channels package's
// Infra interface. It delegates to the host's read-cache invalidator so the
// channels service can bust the cache after mutations.
func (a *App) InvalidateReadCache() {
	a.invalidateReadCache()
}

// LoadSchedulerRun is the exported adapter for the channels package's Infra
// interface. It forwards to the host's scheduler_runs loader and converts
// the host's private schedulerRunRecord to channels.SchedulerRunRecord.
func (a *App) LoadSchedulerRun(ctx context.Context, jobKey string) (channels.SchedulerRunRecord, error) {
	record, err := a.loadSchedulerRun(ctx, jobKey)
	if err != nil {
		return channels.SchedulerRunRecord{}, err
	}
	return channels.SchedulerRunRecord{
		JobKey:         record.JobKey,
		Status:         record.Status,
		PlannedRunKey:  record.PlannedRunKey,
		NextRunAt:      record.NextRunAt,
		LastRunKey:     record.LastRunKey,
		LastStartedAt:  record.LastStartedAt,
		LastFinishedAt: record.LastFinishedAt,
		LastSuccessAt:  record.LastSuccessAt,
		LastError:      record.LastError,
		Summary:        record.Summary,
		UpdatedAt:      record.UpdatedAt,
	}, nil
}

// LoadCheckinScheduleConfig is the exported adapter for the channels
// package's Infra interface. It forwards to the host's checkin.schedule
// loader and converts the host's private checkinScheduleConfig to
// channels.CheckinScheduleConfig.
func (a *App) LoadCheckinScheduleConfig(ctx context.Context) channels.CheckinScheduleConfig {
	config := a.loadCheckinScheduleConfig(ctx)
	return channels.CheckinScheduleConfig{
		Enabled:                config.Enabled,
		Time:                   config.Time,
		RandomDelayMinutes:     config.RandomDelayMinutes,
		SiteConcurrency:        config.SiteConcurrency,
		GlobalConcurrency:      config.GlobalConcurrency,
		SiteMinIntervalSeconds: config.SiteMinIntervalSeconds,
	}
}

// SafeNormalizeBaseURL is the exported adapter for the channels package's
// Infra interface. It validates and normalizes raw against the host's
// outbound URL policy (SSRF defences, controlled by allowLocalOutbound).
func (a *App) SafeNormalizeBaseURL(ctx context.Context, raw string) (string, error) {
	return safeNormalizeBaseURL(ctx, raw, a.externalURLPolicy())
}
