// Package channels implements the relay-channel domain: channel CRUD,
// source-sync status transitions, upstream detection wiring, per-channel
// model sync, channel health overview, per-site checkin scheduling, and
// model pricing source extraction.
//
// The host application (e.g. *core.App) satisfies the Infra interface and
// delegates its *App handler/forwarding methods to this Service. Shared
// API response types (ImportedChannel, ChannelSchedule, ScheduleCalendarItem,
// ChannelHealthOverview, etc.) stay in the host package; this package
// defines local mirror types so it can build them without importing core.
package channels

// ImportedChannel is the channels-package-local mirror of core.ImportedChannel.
// The host keeps core.ImportedChannel in core/models.go per the extraction
// contract; channels.Service produces ImportedChannel values and the host's
// forwarding helpers convert them back to core.ImportedChannel so existing
// call sites are unchanged.
type ImportedChannel struct {
	ID                 string   `json:"id"`
	LocalInstanceID    string   `json:"localInstanceId,omitempty"`
	SourceChannelID    string   `json:"sourceChannelId"`
	SourceType         string   `json:"sourceType,omitempty"`
	Name               string   `json:"name"`
	BaseURL            string   `json:"baseUrl,omitempty"`
	Status             string   `json:"status,omitempty"`
	UpstreamKind       string   `json:"upstreamKind"`
	SupportsCheckin    bool     `json:"supportsCheckin"`
	SupportsBalance    bool     `json:"supportsBalance"`
	SupportsModels     bool     `json:"supportsModels"`
	SupportsPricing    bool     `json:"supportsPricing"`
	ChannelKeyMasked   string   `json:"channelKeyMasked,omitempty"`
	ModelCount         int      `json:"modelCount"`
	SampleModels       []string `json:"sampleModels,omitempty"`
	ModelsSource       string   `json:"modelsSource,omitempty"`
	ModelsStatus       string   `json:"modelsStatus,omitempty"`
	ModelsLastSyncedAt string   `json:"modelsLastSyncedAt,omitempty"`
	ModelsMessage      string   `json:"modelsMessage,omitempty"`
	SourceSyncStatus   string   `json:"sourceSyncStatus,omitempty"`
	SourceMissingAt    string   `json:"sourceMissingAt,omitempty"`
	RawJSON            string   `json:"rawJson,omitempty"`
	DetectionJSON      string   `json:"detectionJson,omitempty"`
	LastDetectedAt     string   `json:"lastDetectedAt,omitempty"`
	CreatedAt          string   `json:"createdAt"`
	UpdatedAt          string   `json:"updatedAt"`
}

// ChannelSchedule is the channels-package-local mirror of
// core.ChannelSchedule. The host keeps core.ChannelSchedule so existing
// JSON shapes and call sites are unchanged.
type ChannelSchedule struct {
	ID             string   `json:"id"`
	UpstreamSiteID string   `json:"upstreamSiteId"`
	SiteName       string   `json:"siteName,omitempty"`
	Enabled        bool     `json:"enabled"`
	CheckinTime    string   `json:"checkinTime"`
	CronExpr       string   `json:"cronExpr"`
	SkipDates      []string `json:"skipDates"`
	RandomDelayMin int      `json:"randomDelayMin"`
	RandomDelayMax int      `json:"randomDelayMax"`
	LastRunAt      string   `json:"lastRunAt,omitempty"`
	NextRunAt      string   `json:"nextRunAt,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// ScheduleCalendarItem is the channels-package-local mirror of
// core.ScheduleCalendarItem.
type ScheduleCalendarItem struct {
	Date     string `json:"date"`
	Time     string `json:"time"`
	SiteName string `json:"siteName"`
	SiteID   string `json:"siteId"`
	JobType  string `json:"jobType"`
	Enabled  bool   `json:"enabled"`
}

// ChannelHealthOverview is the channels-package-local mirror of
// core.ChannelHealthOverview.
type ChannelHealthOverview struct {
	GeneratedAt                string              `json:"generatedAt"`
	Overall                    string              `json:"overall"`
	SiteCount                  int                 `json:"siteCount"`
	HealthySiteCount           int                 `json:"healthySiteCount"`
	UnreachableSiteCount       int                 `json:"unreachableSiteCount"`
	ChannelCount               int                 `json:"channelCount"`
	LiveModelChannelCount      int                 `json:"liveModelChannelCount"`
	FailedModelChannelCount    int                 `json:"failedModelChannelCount"`
	UncheckedModelChannelCount int                 `json:"uncheckedModelChannelCount"`
	ValidKeyCount              int                 `json:"validKeyCount"`
	InvalidKeyCount            int                 `json:"invalidKeyCount"`
	UncheckedKeyCount          int                 `json:"uncheckedKeyCount"`
	Sites                      []ChannelHealthSite `json:"sites"`
}

// ChannelHealthSite is the channels-package-local mirror of
// core.ChannelHealthSite.
type ChannelHealthSite struct {
	SiteID                     string   `json:"siteId"`
	SiteName                   string   `json:"siteName"`
	BaseURL                    string   `json:"baseUrl"`
	Kind                       string   `json:"kind"`
	Level                      string   `json:"level"`
	HealthStatus               string   `json:"healthStatus"`
	AccountCount               int      `json:"accountCount"`
	ValidKeyCount              int      `json:"validKeyCount"`
	InvalidKeyCount            int      `json:"invalidKeyCount"`
	UncheckedKeyCount          int      `json:"uncheckedKeyCount"`
	ModelChannelCount          int      `json:"modelChannelCount"`
	LiveModelChannelCount      int      `json:"liveModelChannelCount"`
	FailedModelChannelCount    int      `json:"failedModelChannelCount"`
	UncheckedModelChannelCount int      `json:"uncheckedModelChannelCount"`
	ModelCount                 int      `json:"modelCount"`
	LastCheckedAt              string   `json:"lastCheckedAt,omitempty"`
	Message                    string   `json:"message,omitempty"`
	RecommendedAction          string   `json:"recommendedAction,omitempty"`
	Samples                    []string `json:"samples,omitempty"`
}

// Detection is the channels-package-local mirror of core.UpstreamDetection.
// Used by Service.DetectChannel to return the detection result without
// importing core.
type Detection struct {
	BaseURL             string   `json:"baseUrl"`
	HomepageURL         string   `json:"homepageUrl"`
	LoginURL            string   `json:"loginUrl"`
	Kind                string   `json:"kind"`
	HealthStatus        string   `json:"healthStatus"`
	DetectionConfidence float64  `json:"detectionConfidence"`
	SupportsCheckin     bool     `json:"supportsCheckin"`
	SupportsBalance     bool     `json:"supportsBalance"`
	SupportsModels      bool     `json:"supportsModels"`
	SupportsPricing     bool     `json:"supportsPricing"`
	MatchedSignals      []string `json:"matchedSignals"`
}

// EnsureSiteInput is the channels-package-local mirror of sites.EnsureSiteInput.
// Used by Infra.EnsureUpstreamSiteForChannel so the channels service can
// request site upserts without importing sites.
type EnsureSiteInput struct {
	ChannelID  string
	Name       string
	RawBaseURL string
	Kind       string
	Detection  *Detection
}

// SetChannelSourceSyncStatusResult is the return value of
// Service.SetChannelSourceSyncStatus. It carries the data the host handler
// needs to build its JSON response.
type SetChannelSourceSyncStatusResult struct {
	ID             string
	Name           string
	PreviousStatus string
	SourceStatus   string
	ChangedAt      string
}

// DetectChannelResult is the return value of Service.DetectChannel.
type DetectChannelResult struct {
	Detection Detection
	SiteID    string
	Created   bool
}

// ChannelModelSyncOverview aggregates per-channel model sync outcomes.
type ChannelModelSyncOverview struct {
	GeneratedAt    string                     `json:"generatedAt"`
	SyncedChannels int                        `json:"syncedChannels,omitempty"`
	ChannelCount   int                        `json:"channelCount"`
	ModelCount     int                        `json:"modelCount"`
	LiveKeyCount   int                        `json:"liveKeyCount"`
	RawOnlyCount   int                        `json:"rawOnlyCount"`
	FailedCount    int                        `json:"failedCount"`
	UncheckedCount int                        `json:"uncheckedCount"`
	Items          []ChannelModelSyncItem     `json:"items"`
	Models         []ChannelModelCoverageItem `json:"models"`
}

// ChannelModelSyncItem describes one channel's model sync outcome.
type ChannelModelSyncItem struct {
	ChannelID    string   `json:"channelId"`
	ChannelName  string   `json:"channelName"`
	BaseURL      string   `json:"baseUrl,omitempty"`
	Kind         string   `json:"kind"`
	HasKey       bool     `json:"hasKey"`
	Status       string   `json:"status"`
	Source       string   `json:"source,omitempty"`
	ModelCount   int      `json:"modelCount"`
	SampleModels []string `json:"sampleModels,omitempty"`
	LatencyMs    int64    `json:"latencyMs,omitempty"`
	Message      string   `json:"message,omitempty"`
	LastSyncedAt string   `json:"lastSyncedAt,omitempty"`
}

// ChannelModelCoverageItem aggregates coverage for a single model across channels.
type ChannelModelCoverageItem struct {
	Model        string   `json:"model"`
	ChannelCount int      `json:"channelCount"`
	LiveKeyCount int      `json:"liveKeyCount"`
	Channels     []string `json:"channels,omitempty"`
}

// ChannelModelSyncRecord is the persisted state for one channel during a
// model sync batch. Loaded by Service.LoadChannelModelSyncRecords and
// consumed by Service.SyncChannelModels.
type ChannelModelSyncRecord struct {
	ID                  string
	Name                string
	BaseURL             string
	Kind                string
	RawJSON             string
	ChannelKeyEncrypted string
	ModelCount          int
	SampleModelsJSON    string
	ModelsSource        string
	ModelsStatus        string
	ModelsLastSyncedAt  string
	ModelsMessage       string
}

// ChannelHealthSiteRow is the raw DB row for the channel health site summary.
type ChannelHealthSiteRow struct {
	SiteID            string
	SiteName          string
	BaseURL           string
	Kind              string
	HealthStatus      string
	LastCheckedAt     string
	AccountCount      int
	ValidKeyCount     int
	InvalidKeyCount   int
	UncheckedKeyCount int
}

// ChannelHealthModelRow is the raw DB row for the channel health model summary.
type ChannelHealthModelRow struct {
	SiteID       string
	BaseURL      string
	Status       string
	ModelCount   int
	Message      string
	LastSyncedAt string
	ChannelName  string
}

// ModelOverview aggregates model coverage across accounts and sites.
type ModelOverview struct {
	GeneratedAt      string                  `json:"generatedAt"`
	SyncedAccounts   int                     `json:"syncedAccounts,omitempty"`
	ModelCount       int                     `json:"modelCount"`
	AccountCount     int                     `json:"accountCount"`
	ValidKeyCount    int                     `json:"validKeyCount"`
	UsableModelCount int                     `json:"usableModelCount"`
	FastestLatencyMs int64                   `json:"fastestLatencyMs,omitempty"`
	Models           []ModelCoverageItem     `json:"models"`
	Sites            []SiteModelCoverageItem `json:"sites"`
	PriceHints       []ModelPriceHint        `json:"priceHints"`
}

// ModelCoverageItem describes coverage for one model across accounts/sites.
type ModelCoverageItem struct {
	Model            string   `json:"model"`
	AccountCount     int      `json:"accountCount"`
	ValidKeyCount    int      `json:"validKeyCount"`
	UsableCount      int      `json:"usableCount"`
	FastestLatencyMs int64    `json:"fastestLatencyMs,omitempty"`
	Sites            []string `json:"sites,omitempty"`
	Fingerprints     []string `json:"fingerprints,omitempty"`
}

// SiteModelCoverageItem describes per-site model coverage.
type SiteModelCoverageItem struct {
	SiteID           string   `json:"siteId"`
	SiteName         string   `json:"siteName"`
	BaseURL          string   `json:"baseUrl"`
	Kind             string   `json:"kind"`
	ModelCount       int      `json:"modelCount"`
	ValidKeyCount    int      `json:"validKeyCount"`
	UsableKeyCount   int      `json:"usableKeyCount"`
	FastestLatencyMs int64    `json:"fastestLatencyMs,omitempty"`
	SampleModels     []string `json:"sampleModels,omitempty"`
}

// ModelPriceHint is a lightweight vendor/level hint inferred from model name.
type ModelPriceHint struct {
	Model      string `json:"model"`
	Vendor     string `json:"vendor"`
	PriceLevel string `json:"priceLevel"`
	Notes      string `json:"notes"`
}

// ModelPricingOverview aggregates raw + live pricing sources.
type ModelPricingOverview struct {
	GeneratedAt      string                 `json:"generatedAt"`
	SourceCount      int                    `json:"sourceCount"`
	ModelCount       int                    `json:"modelCount"`
	ExactCount       int                    `json:"exactCount"`
	RatioCount       int                    `json:"ratioCount"`
	LiveCacheCount   int                    `json:"liveCacheCount"`
	FailedCacheCount int                    `json:"failedCacheCount"`
	Sources          []ModelPricingSource   `json:"sources"`
	SiteCaches       []SitePricingCacheItem `json:"siteCaches"`
	Comparisons      []ModelPriceComparison `json:"comparisons"`
}

// ModelPricingSource describes one pricing source for one model.
type ModelPricingSource struct {
	ChannelID       string   `json:"channelId"`
	ChannelName     string   `json:"channelName"`
	BaseURL         string   `json:"baseUrl,omitempty"`
	Kind            string   `json:"kind"`
	Model           string   `json:"model"`
	UpstreamModel   string   `json:"upstreamModel,omitempty"`
	Source          string   `json:"source"`
	FieldPath       string   `json:"fieldPath"`
	Price           *float64 `json:"price,omitempty"`
	PromptRatio     *float64 `json:"promptRatio,omitempty"`
	CompletionRatio *float64 `json:"completionRatio,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	Confidence      string   `json:"confidence"`
	Notes           string   `json:"notes,omitempty"`
	RawValueMasked  string   `json:"rawValueMasked,omitempty"`
}

// SitePricingCacheItem is one cached /api/pricing probe result.
type SitePricingCacheItem struct {
	SiteID       string `json:"siteId"`
	SiteName     string `json:"siteName"`
	BaseURL      string `json:"baseUrl"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	HTTPStatus   int    `json:"httpStatus,omitempty"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
	SourcePath   string `json:"sourcePath"`
	SourceCount  int    `json:"sourceCount"`
	ModelCount   int    `json:"modelCount"`
	Message      string `json:"message,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
}

// ModelPriceComparison compares pricing/latency/usability across sources for one model.
type ModelPriceComparison struct {
	Model                 string   `json:"model"`
	SourceCount           int      `json:"sourceCount"`
	SiteCount             int      `json:"siteCount"`
	UsableAccountCount    int      `json:"usableAccountCount"`
	FastestLatencyMs      int64    `json:"fastestLatencyMs,omitempty"`
	LowestPrice           *float64 `json:"lowestPrice,omitempty"`
	LowestPromptRatio     *float64 `json:"lowestPromptRatio,omitempty"`
	LowestCompletionRatio *float64 `json:"lowestCompletionRatio,omitempty"`
	BestSource            string   `json:"bestSource,omitempty"`
	Sites                 []string `json:"sites,omitempty"`
	Notes                 string   `json:"notes,omitempty"`
}

// PricingSiteRecord is the raw DB row for one pricing sync target site.
type PricingSiteRecord struct {
	SiteID   string
	SiteName string
	BaseURL  string
	Kind     string
}

// AccountModelRecord is the joined channel_accounts + upstream_sites row
// used by model overview, pricing overview, and key export preview.
type AccountModelRecord struct {
	AccountID     string
	AccountName   string
	SiteID        string
	SiteName      string
	BaseURL       string
	Kind          string
	Fingerprint   string
	Status        string
	ModelCount    int
	SampleModels  []string
	TestModel     string
	ModelUsable   bool
	LatencyMs     int64
	LastCheckedAt string
}

// CheckinScheduleConfig is the channels-package-local mirror of the host's
// checkin.schedule system setting. Used by EnsureGlobalScheduleRecord and
// SyncGlobalScheduleRecord.
type CheckinScheduleConfig struct {
	Enabled                bool   `json:"enabled"`
	Time                   string `json:"time"`
	RandomDelayMinutes     []int  `json:"randomDelayMinutes"`
	SiteConcurrency        int    `json:"siteConcurrency"`
	GlobalConcurrency      int    `json:"globalConcurrency"`
	SiteMinIntervalSeconds int    `json:"siteMinIntervalSeconds"`
}

// SchedulerRunRecord is the channels-package-local mirror of the host's
// schedulerRunRecord. Used by NextSyncCalendarItem to read the next sync run.
type SchedulerRunRecord struct {
	JobKey         string
	Status         string
	PlannedRunKey  string
	NextRunAt      string
	LastRunKey     string
	LastStartedAt  string
	LastFinishedAt string
	LastSuccessAt  string
	LastError      string
	Summary        string
	UpdatedAt      string
}
