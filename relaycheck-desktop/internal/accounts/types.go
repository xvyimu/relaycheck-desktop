package accounts

// Detection mirrors core.UpstreamDetection. It is the upstream-site probe
// result returned by Infra.DetectUpstreamForImport and consumed by the
// import/sync flows when persisting imported_channels / upstream_sites rows.
// The host (e.g. *core.App) converts between this mirror and its private
// core.UpstreamDetection type.
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

// LocalNewAPIInstance mirrors core.LocalNewAPIInstance. It describes a local
// NewAPI/OneAPI instance row from local_newapi_instances, used by the
// instance-management and sync-preview flows.
type LocalNewAPIInstance struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	BaseURL            string `json:"baseUrl"`
	DetectedFrom       string `json:"detectedFrom,omitempty"`
	Status             string `json:"status"`
	Version            string `json:"version,omitempty"`
	DatabasePath       string `json:"databasePath,omitempty"`
	ChannelCount       int    `json:"channelCount"`
	HasSyncToken       bool   `json:"hasSyncToken"`
	SyncTokenMasked    string `json:"syncTokenMasked,omitempty"`
	LastScannedAt      string `json:"lastScannedAt,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	SyncCapability     string `json:"syncCapability"`
	SyncTokenEncrypted string `json:"-"`
}

// SyncPreview mirrors core.LocalNewAPISyncPreview. It shows what would change
// if a sync is performed against a local NewAPI instance.
type SyncPreview struct {
	InstanceID     string            `json:"instanceId"`
	InstanceName   string            `json:"instanceName"`
	Source         string            `json:"source"`
	Total          int               `json:"total"`
	NewCount       int               `json:"newCount"`
	ChangedCount   int               `json:"changedCount"`
	UnchangedCount int               `json:"unchangedCount"`
	SkippedCount   int               `json:"skippedCount"`
	RemovedCount   int               `json:"removedCount"`
	Items          []SyncPreviewItem `json:"items"`
	GeneratedAt    string            `json:"generatedAt"`
}

// SyncPreviewItem mirrors core.SyncPreviewItem. Action is
// "new" | "changed" | "unchanged" | "skipped" | "removed".
type SyncPreviewItem struct {
	SourceChannelID string   `json:"sourceChannelId"`
	Name            string   `json:"name"`
	BaseURL         string   `json:"baseUrl,omitempty"`
	Status          string   `json:"status,omitempty"`
	UpstreamKind    string   `json:"upstreamKind"`
	Action          string   `json:"action"`
	Reason          string   `json:"reason"`
	ChangedFields   []string `json:"changedFields,omitempty"`
}

// SyncRunInput mirrors core.localNewAPISyncRunInput. It is the request body
// for syncing a local NewAPI instance (SQLite path or admin API).
type SyncRunInput struct {
	AccessToken       string `json:"accessToken"`
	SaveAccessToken   bool   `json:"saveAccessToken"`
	ClearAccessToken  bool   `json:"clearAccessToken"`
	UserID            string `json:"userId"`
	ImportKeys        bool   `json:"importKeys"`
	SkipCreateSites   bool   `json:"skipCreateSites"`
	DetectAfterImport bool   `json:"detectAfterImport"`
	PageSize          int    `json:"pageSize"`
}

// SyncSourceInput mirrors core.localNewAPISyncSourceInput. It is the request
// body for the sync-preview / mark-missing flows (no import-keys flag).
type SyncSourceInput struct {
	AccessToken      string `json:"accessToken"`
	SaveAccessToken  bool   `json:"saveAccessToken"`
	ClearAccessToken bool   `json:"clearAccessToken"`
	UserID           string `json:"userId"`
	PageSize         int    `json:"pageSize"`
}

// AutoDetectResult mirrors core.autoDetectResult. It is one DB's outcome in
// the auto-detect-and-import flow.
type AutoDetectResult struct {
	DBPath        string `json:"dbPath"`
	BaseURL       string `json:"baseUrl"`
	ImportedCount int    `json:"importedCount"`
	SitesCreated  int    `json:"sitesCreated"`
	SitesMerged   int    `json:"sitesMerged"`
	Error         string `json:"error,omitempty"`
}

// chromePasswordRow is a parsed Chrome password CSV entry.
type chromePasswordRow struct {
	Name     string
	URL      string
	Username string
	Password string
}

// passwordSite is an upstream_sites row used for Chrome password matching.
type passwordSite struct {
	ID      string
	Name    string
	BaseURL string
	Host    string
}

// chromePasswordMatch is a matched (site, chrome row) pair for preview/import.
type chromePasswordMatch struct {
	SiteID          string `json:"siteId"`
	SiteName        string `json:"siteName"`
	SiteBaseURL     string `json:"siteBaseUrl"`
	ChromeName      string `json:"chromeName"`
	URL             string `json:"url"`
	Username        string `json:"username"`
	PasswordMasked  string `json:"passwordMasked"`
	ExistingAccount bool   `json:"existingAccount"`
}

// legacySiteConfig is the JSON shape of a legacy config_site*.json file.
type legacySiteConfig struct {
	Name       string `json:"name"`
	SiteName   string `json:"site_name"`
	BaseURL    string `json:"base_url"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	LoginURL   string `json:"login_url"`
	CheckinURL string `json:"checkin_url"`
	BalanceURL string `json:"balance_url"`
}

// existingImportedChannel is the subset of imported_channels fields used by
// the sync-preview diff.
type existingImportedChannel struct {
	Name      string
	BaseURL   string
	Status    string
	Kind      string
	RawJSON   string
	KeyMasked string
}

// preparedSyncRecord is a normalised source-channel record ready for diffing.
type preparedSyncRecord struct {
	SourceID  string
	Name      string
	BaseURL   string
	Status    string
	Kind      string
	RawJSON   string
	KeyMasked string
}
