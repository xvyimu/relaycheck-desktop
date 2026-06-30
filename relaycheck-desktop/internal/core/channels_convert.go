package core

import (
	"relaycheck-desktop/internal/channels"
)

// This file hosts the type converters between core's shared API response
// types (ImportedChannel, ChannelSchedule, ScheduleCalendarItem,
// ChannelHealthOverview, etc.) and the channels package's local mirror
// types. The channels service returns mirror types; the host's *App
// forwarders convert them back to core types so existing call sites and
// JSON shapes are unchanged.
//
// Converters are intentionally explicit (field-by-field) rather than
// json-round-tripped: the round trip would silently drop fields on either
// side and add non-trivial overhead on hot paths like ListChannels.

func channelFromMirror(item channels.ImportedChannel) ImportedChannel {
	return ImportedChannel{
		ID:                 item.ID,
		LocalInstanceID:    item.LocalInstanceID,
		SourceChannelID:    item.SourceChannelID,
		SourceType:         item.SourceType,
		Name:               item.Name,
		BaseURL:            item.BaseURL,
		Status:             item.Status,
		UpstreamKind:       item.UpstreamKind,
		SupportsCheckin:    item.SupportsCheckin,
		SupportsBalance:    item.SupportsBalance,
		SupportsModels:     item.SupportsModels,
		SupportsPricing:    item.SupportsPricing,
		ChannelKeyMasked:   item.ChannelKeyMasked,
		ModelCount:         item.ModelCount,
		SampleModels:       item.SampleModels,
		ModelsSource:       item.ModelsSource,
		ModelsStatus:       item.ModelsStatus,
		ModelsLastSyncedAt: item.ModelsLastSyncedAt,
		ModelsMessage:      item.ModelsMessage,
		SourceSyncStatus:   item.SourceSyncStatus,
		SourceMissingAt:    item.SourceMissingAt,
		RawJSON:            item.RawJSON,
		DetectionJSON:      item.DetectionJSON,
		LastDetectedAt:     item.LastDetectedAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func channelsListToCore(items []channels.ImportedChannel) []ImportedChannel {
	if len(items) == 0 {
		return nil
	}
	out := make([]ImportedChannel, 0, len(items))
	for _, item := range items {
		out = append(out, channelFromMirror(item))
	}
	return out
}

func scheduleFromMirror(item channels.ChannelSchedule) ChannelSchedule {
	return ChannelSchedule{
		ID:             item.ID,
		UpstreamSiteID: item.UpstreamSiteID,
		SiteName:       item.SiteName,
		Enabled:        item.Enabled,
		CheckinTime:    item.CheckinTime,
		CronExpr:       item.CronExpr,
		SkipDates:      item.SkipDates,
		RandomDelayMin: item.RandomDelayMin,
		RandomDelayMax: item.RandomDelayMax,
		LastRunAt:      item.LastRunAt,
		NextRunAt:      item.NextRunAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func schedulesToCore(items []channels.ChannelSchedule) []ChannelSchedule {
	if len(items) == 0 {
		return nil
	}
	out := make([]ChannelSchedule, 0, len(items))
	for _, item := range items {
		out = append(out, scheduleFromMirror(item))
	}
	return out
}

func calendarItemFromMirror(item channels.ScheduleCalendarItem) ScheduleCalendarItem {
	return ScheduleCalendarItem{
		Date:     item.Date,
		Time:     item.Time,
		SiteName: item.SiteName,
		SiteID:   item.SiteID,
		JobType:  item.JobType,
		Enabled:  item.Enabled,
	}
}

func calendarItemsToCore(items []channels.ScheduleCalendarItem) []ScheduleCalendarItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]ScheduleCalendarItem, 0, len(items))
	for _, item := range items {
		out = append(out, calendarItemFromMirror(item))
	}
	return out
}

// scheduleToMirror converts a core.ChannelSchedule to the channels mirror
// type. Used by handleScheduleCalendar which iterates schedules returned by
// listChannelSchedules (core type) and feeds them to channels.CalendarItemsForSchedule.
func scheduleToMirror(item ChannelSchedule) channels.ChannelSchedule {
	return channels.ChannelSchedule{
		ID:             item.ID,
		UpstreamSiteID: item.UpstreamSiteID,
		SiteName:       item.SiteName,
		Enabled:        item.Enabled,
		CheckinTime:    item.CheckinTime,
		CronExpr:       item.CronExpr,
		SkipDates:      item.SkipDates,
		RandomDelayMin: item.RandomDelayMin,
		RandomDelayMax: item.RandomDelayMax,
		LastRunAt:      item.LastRunAt,
		NextRunAt:      item.NextRunAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

// detectionFromMirror converts a channels.Detection back to core.UpstreamDetection.
// Used by detectChannel host handler so the JSON response keeps the same
// core.UpstreamDetection shape other call sites and tests rely on.
func detectionFromMirror(d channels.Detection) UpstreamDetection {
	return UpstreamDetection{
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
	}
}

// channelModelSyncRecordToMirror converts a core channelModelSyncRecord to the
// channels mirror type. Used by syncChannelModels forwarder so the channels
// service receives the record without importing core.
func channelModelSyncRecordToMirror(r channelModelSyncRecord) channels.ChannelModelSyncRecord {
	return channels.ChannelModelSyncRecord{
		ID:                  r.ID,
		Name:                r.Name,
		BaseURL:             r.BaseURL,
		Kind:                r.Kind,
		RawJSON:             r.RawJSON,
		ChannelKeyEncrypted: r.ChannelKeyEncrypted,
		ModelCount:          r.ModelCount,
		SampleModelsJSON:    r.SampleModelsJSON,
		ModelsSource:        r.ModelsSource,
		ModelsStatus:        r.ModelsStatus,
		ModelsLastSyncedAt:  r.ModelsLastSyncedAt,
		ModelsMessage:       r.ModelsMessage,
	}
}

// channelModelSyncItemFromMirror converts a channels.ChannelModelSyncItem back
// to core channelModelSyncItem. Used by syncChannelModels/loadChannelModelItems
// forwarders so existing callers (tests, task_runner) see the same core type.
func channelModelSyncItemFromMirror(m channels.ChannelModelSyncItem) channelModelSyncItem {
	return channelModelSyncItem{
		ChannelID:    m.ChannelID,
		ChannelName:  m.ChannelName,
		BaseURL:      m.BaseURL,
		Kind:         m.Kind,
		HasKey:       m.HasKey,
		Status:       m.Status,
		Source:       m.Source,
		ModelCount:   m.ModelCount,
		SampleModels: m.SampleModels,
		LatencyMs:    m.LatencyMs,
		Message:      m.Message,
		LastSyncedAt: m.LastSyncedAt,
	}
}

// channelModelSyncItemsToCore converts a slice of channels.ChannelModelSyncItem
// to core channelModelSyncItem. Used by loadChannelModelItems forwarder.
func channelModelSyncItemsToCore(items []channels.ChannelModelSyncItem) []channelModelSyncItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]channelModelSyncItem, 0, len(items))
	for _, item := range items {
		out = append(out, channelModelSyncItemFromMirror(item))
	}
	return out
}

// channelModelSyncItemsToMirror converts a slice of core channelModelSyncItem
// to channels.ChannelModelSyncItem. Used by handleChannelModelsSync to feed
// core items into channels.BuildChannelModelOverview.
func channelModelSyncItemsToMirror(items []channelModelSyncItem) []channels.ChannelModelSyncItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]channels.ChannelModelSyncItem, 0, len(items))
	for _, item := range items {
		out = append(out, channels.ChannelModelSyncItem{
			ChannelID:    item.ChannelID,
			ChannelName:  item.ChannelName,
			BaseURL:      item.BaseURL,
			Kind:         item.Kind,
			HasKey:       item.HasKey,
			Status:       item.Status,
			Source:       item.Source,
			ModelCount:   item.ModelCount,
			SampleModels: item.SampleModels,
			LatencyMs:    item.LatencyMs,
			Message:      item.Message,
			LastSyncedAt: item.LastSyncedAt,
		})
	}
	return out
}

func channelHealthOverviewFromMirror(o channels.ChannelHealthOverview) ChannelHealthOverview {
	sites := make([]ChannelHealthSite, 0, len(o.Sites))
	for _, s := range o.Sites {
		sites = append(sites, channelHealthSiteFromMirror(s))
	}
	return ChannelHealthOverview{
		GeneratedAt:                o.GeneratedAt,
		Overall:                    o.Overall,
		SiteCount:                  o.SiteCount,
		HealthySiteCount:           o.HealthySiteCount,
		UnreachableSiteCount:       o.UnreachableSiteCount,
		ChannelCount:               o.ChannelCount,
		LiveModelChannelCount:      o.LiveModelChannelCount,
		FailedModelChannelCount:    o.FailedModelChannelCount,
		UncheckedModelChannelCount: o.UncheckedModelChannelCount,
		ValidKeyCount:              o.ValidKeyCount,
		InvalidKeyCount:            o.InvalidKeyCount,
		UncheckedKeyCount:          o.UncheckedKeyCount,
		Sites:                      sites,
	}
}

func channelHealthSiteFromMirror(s channels.ChannelHealthSite) ChannelHealthSite {
	return ChannelHealthSite{
		SiteID:                     s.SiteID,
		SiteName:                   s.SiteName,
		BaseURL:                    s.BaseURL,
		Kind:                       s.Kind,
		Level:                      s.Level,
		HealthStatus:               s.HealthStatus,
		AccountCount:               s.AccountCount,
		ValidKeyCount:              s.ValidKeyCount,
		InvalidKeyCount:            s.InvalidKeyCount,
		UncheckedKeyCount:          s.UncheckedKeyCount,
		ModelChannelCount:          s.ModelChannelCount,
		LiveModelChannelCount:      s.LiveModelChannelCount,
		FailedModelChannelCount:    s.FailedModelChannelCount,
		UncheckedModelChannelCount: s.UncheckedModelChannelCount,
		ModelCount:                 s.ModelCount,
		LastCheckedAt:              s.LastCheckedAt,
		Message:                    s.Message,
		RecommendedAction:          s.RecommendedAction,
		Samples:                    s.Samples,
	}
}

// pricingSourceFromMirror converts a channels.ModelPricingSource back to the
// core modelPricingSource. *float64 fields are copied by pointer value; this
// is safe because both sides treat them as read-only after construction.
func pricingSourceFromMirror(s channels.ModelPricingSource) modelPricingSource {
	return modelPricingSource{
		ChannelID:       s.ChannelID,
		ChannelName:     s.ChannelName,
		BaseURL:         s.BaseURL,
		Kind:            s.Kind,
		Model:           s.Model,
		UpstreamModel:   s.UpstreamModel,
		Source:          s.Source,
		FieldPath:       s.FieldPath,
		Price:           s.Price,
		PromptRatio:     s.PromptRatio,
		CompletionRatio: s.CompletionRatio,
		Unit:            s.Unit,
		Currency:        s.Currency,
		Confidence:      s.Confidence,
		Notes:           s.Notes,
		RawValueMasked:  s.RawValueMasked,
	}
}

// pricingSourcesToCore converts a slice of channels.ModelPricingSource to the
// core modelPricingSource slice. Used by loadRawChannelPricingCache and
// loadSitePricingCache forwarders.
func pricingSourcesToCore(items []channels.ModelPricingSource) []modelPricingSource {
	if len(items) == 0 {
		return nil
	}
	out := make([]modelPricingSource, 0, len(items))
	for _, item := range items {
		out = append(out, pricingSourceFromMirror(item))
	}
	return out
}

// sitePricingCacheItemFromMirror converts a channels.SitePricingCacheItem back
// to core sitePricingCacheItem. Used by syncSitePricing and loadSitePricingCache
// forwarders so existing callers see the same core type.
func sitePricingCacheItemFromMirror(m channels.SitePricingCacheItem) sitePricingCacheItem {
	return sitePricingCacheItem{
		SiteID:       m.SiteID,
		SiteName:     m.SiteName,
		BaseURL:      m.BaseURL,
		Kind:         m.Kind,
		Status:       m.Status,
		HTTPStatus:   m.HTTPStatus,
		LatencyMs:    m.LatencyMs,
		SourcePath:   m.SourcePath,
		SourceCount:  m.SourceCount,
		ModelCount:   m.ModelCount,
		Message:      m.Message,
		LastSyncedAt: m.LastSyncedAt,
	}
}

// sitePricingCacheItemsFromMirror converts a slice of channels.SitePricingCacheItem
// to core sitePricingCacheItem. Used by loadSitePricingCache forwarder.
func sitePricingCacheItemsFromMirror(items []channels.SitePricingCacheItem) []sitePricingCacheItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]sitePricingCacheItem, 0, len(items))
	for _, item := range items {
		out = append(out, sitePricingCacheItemFromMirror(item))
	}
	return out
}

// accountModelRecordFromMirror converts a channels.AccountModelRecord back to
// core accountModelRecord. Used by loadAccountModelRecords forwarder so
// existing callers (handlers, key export preview) see the same core type.
func accountModelRecordFromMirror(m channels.AccountModelRecord) accountModelRecord {
	return accountModelRecord{
		AccountID:     m.AccountID,
		AccountName:   m.AccountName,
		SiteID:        m.SiteID,
		SiteName:      m.SiteName,
		BaseURL:       m.BaseURL,
		Kind:          m.Kind,
		Fingerprint:   m.Fingerprint,
		Status:        m.Status,
		ModelCount:    m.ModelCount,
		SampleModels:  m.SampleModels,
		TestModel:     m.TestModel,
		ModelUsable:   m.ModelUsable,
		LatencyMs:     m.LatencyMs,
		LastCheckedAt: m.LastCheckedAt,
	}
}

// accountModelRecordsToCore converts a slice of channels.AccountModelRecord to
// core accountModelRecord. Used by loadAccountModelRecords forwarder.
func accountModelRecordsToCore(items []channels.AccountModelRecord) []accountModelRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]accountModelRecord, 0, len(items))
	for _, item := range items {
		out = append(out, accountModelRecordFromMirror(item))
	}
	return out
}

// pricingSiteRecordToMirror converts a core pricingSiteRecord to the channels
// mirror type. Used by syncSitePricing forwarder so the channels service
// receives the record without importing core.
func pricingSiteRecordToMirror(r pricingSiteRecord) channels.PricingSiteRecord {
	return channels.PricingSiteRecord{
		SiteID:   r.SiteID,
		SiteName: r.SiteName,
		BaseURL:  r.BaseURL,
		Kind:     r.Kind,
	}
}
