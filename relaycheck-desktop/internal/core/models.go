package core

// DashboardSummary is the high-level count overview shown on the dashboard.
type DashboardSummary struct {
	LocalNewAPICount       int `json:"localNewApiCount"`
	ImportedChannelCount   int `json:"importedChannelCount"`
	IdentifiedChannelCount int `json:"identifiedChannelCount"`
	AccountCount           int `json:"accountCount"`
	UnreadNotifications    int `json:"unreadNotifications"`
}

// SystemDiagnostics holds the latest self-check result with an overall level
// ("ok" | "warning" | "critical") and a list of diagnostic items.
type SystemDiagnostics struct {
	GeneratedAt string           `json:"generatedAt"`
	Overall     string           `json:"overall"`
	Items       []DiagnosticItem `json:"items"`
}

// SchedulerStatus reports all registered background jobs and their current state.
type SchedulerStatus struct {
	GeneratedAt string               `json:"generatedAt"`
	Jobs        []SchedulerJobStatus `json:"jobs"`
}

// SchedulerJobStatus describes one background job (e.g. "checkin.daily").
// Status is one of "idle" | "running" | "succeeded" | "failed".
type SchedulerJobStatus struct {
	Key            string `json:"key"`
	Label          string `json:"label"`
	Status         string `json:"status"`
	PlannedRunKey  string `json:"plannedRunKey,omitempty"`
	NextRunAt      string `json:"nextRunAt,omitempty"`
	LastRunKey     string `json:"lastRunKey,omitempty"`
	LastStartedAt  string `json:"lastStartedAt,omitempty"`
	LastFinishedAt string `json:"lastFinishedAt,omitempty"`
	LastSuccessAt  string `json:"lastSuccessAt,omitempty"`
	LastError      string `json:"lastError,omitempty"`
	Summary        string `json:"summary,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

// SystemStatus is the response payload for /api/system/status.
// It includes product metadata, runtime address, port conflict info,
// proxy/scheduler sub-status, and the dashboard summary.
type SystemStatus struct {
	ProductName     string                  `json:"productName"`
	ProductVersion  string                  `json:"productVersion"`
	BuildTime       string                  `json:"buildTime"`
	Architecture    string                  `json:"architecture"`
	BindAddress     string                  `json:"bindAddress"`
	Port            int                     `json:"port"`
	PreferredPort   int                     `json:"preferredPort"`   // original port from env/config
	PortConflict    bool                    `json:"portConflict"`     // true if PreferredPort was busy and a fallback was used
	DatabasePath    string                  `json:"databasePath"`
	BackupDir       string                  `json:"backupDir"`
	NetworkProxy    NetworkProxyStatus      `json:"networkProxy"`
	Scheduler       SchedulerStatus         `json:"scheduler"`
	LastDiagnostics SystemStatusDiagnostics `json:"lastDiagnostics"`
	Summary         DashboardSummary        `json:"summary"`
}

// SystemStatusDiagnostics is a lightweight summary of the last diagnostic run.
type SystemStatusDiagnostics struct {
	Overall     string `json:"overall"` // "ok" | "warning" | "critical"
	GeneratedAt string `json:"generatedAt"`
	ItemCount   int    `json:"itemCount"`
}

// HealthStatus is the response for /api/health (no auth required).
type HealthStatus struct {
	Status      string        `json:"status"` // "ok" | "degraded" | "down"
	GeneratedAt string        `json:"generatedAt"`
	Checks      []HealthCheck `json:"checks"`
}

// HealthCheck represents a single health probe result.
type HealthCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"` // "ok" | "warn" | "fail"
	Message string `json:"message,omitempty"`
}

// ActionCenter aggregates actionable items from across the system for the
// dashboard banner. Overall is "ok" | "warning" | "critical".
type ActionCenter struct {
	GeneratedAt string       `json:"generatedAt"`
	Overall     string       `json:"overall"`
	Items       []ActionItem `json:"items"`
}

// ActionItem is a single recommended action shown in the Action Center.
// Level is "info" | "warning" | "critical".
type ActionItem struct {
	ID          string   `json:"id"`
	Priority    int      `json:"priority"` // lower = higher priority
	Level       string   `json:"level"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Count       int      `json:"count"`
	Target      string   `json:"target"` // route path for navigation
	Filter      string   `json:"filter,omitempty"`
	Action      string   `json:"action"`
	Samples     []string `json:"samples,omitempty"`
}

// CheckinStatus reports the current state of the checkin engine, including
// real-time progress when a batch is running and today's summary.
type CheckinStatus struct {
	GeneratedAt       string                `json:"generatedAt"`
	Running           bool                  `json:"running"`
	Mode              string                `json:"mode"` // "manual" | "scheduled"
	CurrentAccountID  string                `json:"currentAccountId,omitempty"`
	CurrentAccount    string                `json:"currentAccount,omitempty"`
	CurrentSite       string                `json:"currentSite,omitempty"`
	CurrentMessage    string                `json:"currentMessage,omitempty"`
	TotalAccounts     int                   `json:"totalAccounts"`
	ProcessedAccounts int                   `json:"processedAccounts"`
	PendingAccounts   int                   `json:"pendingAccounts"`
	SuccessCount      int                   `json:"successCount"`
	AlreadyCount      int                   `json:"alreadyCount"`
	FailedCount       int                   `json:"failedCount"`
	UnsupportedCount  int                   `json:"unsupportedCount"`
	AuthExpiredCount  int                   `json:"authExpiredCount"`
	StartedAt         string                `json:"startedAt,omitempty"`
	UpdatedAt         string                `json:"updatedAt,omitempty"`
	FinishedAt        string                `json:"finishedAt,omitempty"`
	LastRunMessage    string                `json:"lastRunMessage,omitempty"`
	Today             CheckinTodaySummary   `json:"today"`
	Schedule          CheckinScheduleStatus `json:"schedule"`
}

// CheckinTodaySummary aggregates today's checkin results.
type CheckinTodaySummary struct {
	TotalLogs        int `json:"totalLogs"`
	SuccessCount     int `json:"successCount"`
	AlreadyCount     int `json:"alreadyCount"`
	FailedCount      int `json:"failedCount"`
	UnsupportedCount int `json:"unsupportedCount"`
	AuthExpiredCount int `json:"authExpiredCount"`
	DueAccounts      int `json:"dueAccounts"`
}

// CheckinScheduleStatus describes the daily checkin schedule.
type CheckinScheduleStatus struct {
	Enabled             bool   `json:"enabled"`
	Time                string `json:"time"` // "HH:MM" format
	RandomDelayMin      int    `json:"randomDelayMin"`
	RandomDelayMax      int    `json:"randomDelayMax"`
	NextRunAt           string `json:"nextRunAt,omitempty"`
	NextWindowStartAt   string `json:"nextWindowStartAt,omitempty"`
	NextWindowEndAt     string `json:"nextWindowEndAt,omitempty"`
	NextRunInSeconds    int64  `json:"nextRunInSeconds"`
	NextWindowInSeconds int64  `json:"nextWindowInSeconds"`
	Message             string `json:"message,omitempty"`
}

// AutoStartStatus reports the current state of the OS-level auto-start
// configuration (Windows shell:startup .lnk shortcut).
type AutoStartStatus struct {
	// Enabled is true when the startup shortcut currently exists on disk.
	Enabled bool `json:"enabled"`
	// Supported is true on platforms where auto-start can be configured
	// (currently Windows only).
	Supported bool `json:"supported"`
	// ShortcutPath is the resolved .lnk path inside the shell:startup folder.
	ShortcutPath string `json:"shortcutPath,omitempty"`
	// TargetPath is the executable the shortcut will launch.
	TargetPath string `json:"targetPath,omitempty"`
	// Error carries the last error message, if any.
	Error string `json:"error,omitempty"`
}

// SystemSetting is a key-value pair stored in the system_settings table.
type SystemSetting struct {
	Key       string `json:"key"`
	ValueJSON string `json:"valueJson"`
	UpdatedAt string `json:"updatedAt"`
}

// SystemBackup describes a database backup file.
type SystemBackup struct {
	FileName  string `json:"fileName"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

// AuditLogItem is a single audit trail entry.
// Level is "info" | "warning" | "critical".
type AuditLogItem struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	Level        string `json:"level"`
	Actor        string `json:"actor,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	ResourceID   string `json:"resourceId,omitempty"`
	Summary      string `json:"summary"`
	MetadataJSON string `json:"metadataJson,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// DiagnosticItem is a single self-check finding.
// Level is "ok" | "warning" | "critical".
type DiagnosticItem struct {
	ID            string   `json:"id"`
	Level         string   `json:"level"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Action        string   `json:"action,omitempty"`
	SolutionSteps []string `json:"solutionSteps,omitempty"`
	Count         int      `json:"count,omitempty"`
}

// LocalNewAPIInstance represents a NewAPI installation detected on the local
// machine. SyncCapability is "none" | "read" | "read_write".
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

// LocalNewAPISyncPreview shows what would change if a sync is performed.
type LocalNewAPISyncPreview struct {
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

// SyncPreviewItem represents one channel's sync action.
// Action is "add" | "update" | "skip" | "remove".
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

// UpstreamSite is a detected API relay site (NewAPI/OneAPI/Sub2API etc.).
// Kind is "newapi" | "oneapi" | "sub2api" | "unknown".
// HealthStatus is "healthy" | "degraded" | "down" | "unknown".
type UpstreamSite struct {
	ID                  string  `json:"id"`
	ChannelID           string  `json:"channelId,omitempty"`
	Name                string  `json:"name"`
	HomepageURL         string  `json:"homepageUrl,omitempty"`
	BaseURL             string  `json:"baseUrl"`
	LoginURL            string  `json:"loginUrl,omitempty"`
	Kind                string  `json:"kind"`
	DetectionConfidence float64 `json:"detectionConfidence"`
	HealthStatus        string  `json:"healthStatus"`
	SupportsCheckin     bool    `json:"supportsCheckin"`
	SupportsBalance     bool    `json:"supportsBalance"`
	SupportsModels      bool    `json:"supportsModels"`
	SupportsPricing     bool    `json:"supportsPricing"`
	AccountCount        int     `json:"accountCount,omitempty"`
	DetectionJSON       string  `json:"detectionJson,omitempty"`
	LastHealthCheckAt   string  `json:"lastHealthCheckAt,omitempty"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

// ImportedChannel is a channel imported from a local NewAPI instance.
// UpstreamKind is "newapi" | "oneapi" | "sub2api" | "unknown".
// Status is "active" | "disabled" | "removed".
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

// SiteDetail is the full detail view for an upstream site, including
// related accounts, recent balance snapshots, and checkin logs.
type SiteDetail struct {
	Site             UpstreamSite      `json:"site"`
	Detection        UpstreamDetection `json:"detection"`
	Accounts         []ChannelAccount  `json:"accounts"`
	BalanceSnapshots []BalanceSnapshot `json:"balanceSnapshots"`
	CheckinLogs      []CheckinLog      `json:"checkinLogs"`
	Suggestions      []string          `json:"suggestions"`
}

// ChannelAccount is the core account entity. It stores credentials (encrypted),
// API key test results, balance, and checkin status.
// AuthType is "cookie" | "token" | "api_key" | "browser_profile".
// LoginStatus is "logged_in" | "logged_out" | "expired" | "unknown".
type ChannelAccount struct {
	ID                   string   `json:"id"`
	UpstreamSiteID       string   `json:"upstreamSiteId"`
	UpstreamSiteName     string   `json:"upstreamSiteName,omitempty"`
	UpstreamSiteBaseURL  string   `json:"upstreamSiteBaseUrl,omitempty"`
	UpstreamSiteLoginURL string   `json:"upstreamSiteLoginUrl,omitempty"`
	UpstreamSiteKind     string   `json:"upstreamSiteKind,omitempty"`
	DisplayName          string   `json:"displayName"`
	Email                string   `json:"email,omitempty"`
	Username             string   `json:"username,omitempty"`
	AuthType             string   `json:"authType"`
	BrowserProfilePath   string   `json:"browserProfilePath,omitempty"`
	LoginStatus          string   `json:"loginStatus"`
	APIKeyFingerprint    string   `json:"apiKeyFingerprint,omitempty"`
	APIKeyStatus         string   `json:"apiKeyStatus,omitempty"` // "valid" | "invalid" | "untested" | "rate_limited"
	APIKeyLastCheckedAt  string   `json:"apiKeyLastCheckedAt,omitempty"`
	APIKeyModelCount     int      `json:"apiKeyModelCount"`
	APIKeySampleModels   []string `json:"apiKeySampleModels,omitempty"`
	APIKeyTestModel      string   `json:"apiKeyTestModel,omitempty"`
	APIKeyModelUsable    bool     `json:"apiKeyModelUsable"`
	APIKeyLatencyMs      int64    `json:"apiKeyLatencyMs,omitempty"`
	APIKeyTestHTTPStatus int      `json:"apiKeyTestHttpStatus,omitempty"`
	APIKeyTestMessage    string   `json:"apiKeyTestMessage,omitempty"`
	APIKeyTestPath       string   `json:"apiKeyTestPath,omitempty"`
	Balance              *float64 `json:"balance,omitempty"`
	BalanceUnit          string   `json:"balanceUnit,omitempty"`
	LastCheckinAt        string   `json:"lastCheckinAt,omitempty"`
	LastCheckinStatus    string   `json:"lastCheckinStatus,omitempty"`
	LastCheckinMessage   string   `json:"lastCheckinMessage,omitempty"`
	LastLoginAt          string   `json:"lastLoginAt,omitempty"`
	LastValidatedAt      string   `json:"lastValidatedAt,omitempty"`
	CookieExpiryAt       string   `json:"cookieExpiryAt,omitempty"`       // estimated cookie expiry time (ISO 8601)
	StorageStateExpiryAt string  `json:"storageStateExpiryAt,omitempty"` // estimated browser storage state expiry
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
}

// CheckinLog records a single checkin attempt.
// Status is "success" | "already" | "failed" | "unsupported" | "auth_expired".
type CheckinLog struct {
	ID                string `json:"id"`
	AccountID         string `json:"accountId"`
	AccountName       string `json:"accountName,omitempty"`
	UpstreamSiteID    string `json:"upstreamSiteId"`
	UpstreamSiteName  string `json:"upstreamSiteName,omitempty"`
	ChannelID         string `json:"channelId,omitempty"`
	Status            string `json:"status"`
	Reward            string `json:"reward,omitempty"`
	Message           string `json:"message,omitempty"`
	RawResponseMasked string `json:"rawResponseMasked,omitempty"`
	StartedAt         string `json:"startedAt"`
	FinishedAt        string `json:"finishedAt"`
}

// BalanceSnapshot records a point-in-time balance query result.
type BalanceSnapshot struct {
	ID                string   `json:"id"`
	AccountID         string   `json:"accountId"`
	AccountName       string   `json:"accountName,omitempty"`
	UpstreamSiteID    string   `json:"upstreamSiteId"`
	UpstreamSiteName  string   `json:"upstreamSiteName,omitempty"`
	ChannelID         string   `json:"channelId,omitempty"`
	Balance           *float64 `json:"balance,omitempty"`
	UsedQuota         *float64 `json:"usedQuota,omitempty"`
	TotalQuota        *float64 `json:"totalQuota,omitempty"`
	Unit              string   `json:"unit"`
	RawResponseMasked string   `json:"rawResponseMasked,omitempty"`
	CreatedAt         string   `json:"createdAt"`
}

// Notification is a user-facing notification shown in the notification center.
// Type is "checkin" | "balance" | "system" | "alert".
// Level is "info" | "warning" | "critical".
type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

// NotificationChannelStatus reports the configuration state of a notification
// channel (e.g. webhook, telegram, email).
type NotificationChannelStatus struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	ConfigValid bool     `json:"configValid"`
	Levels      []string `json:"levels,omitempty"`
}
