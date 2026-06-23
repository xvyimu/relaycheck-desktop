package core

type DashboardSummary struct {
	LocalNewAPICount       int `json:"localNewApiCount"`
	ImportedChannelCount   int `json:"importedChannelCount"`
	IdentifiedChannelCount int `json:"identifiedChannelCount"`
	AccountCount           int `json:"accountCount"`
	UnreadNotifications    int `json:"unreadNotifications"`
}

type SystemDiagnostics struct {
	GeneratedAt string           `json:"generatedAt"`
	Overall     string           `json:"overall"`
	Items       []DiagnosticItem `json:"items"`
}

type SchedulerStatus struct {
	GeneratedAt string               `json:"generatedAt"`
	Jobs        []SchedulerJobStatus `json:"jobs"`
}

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

type SystemStatus struct {
	ProductName     string                  `json:"productName"`
	ProductVersion  string                  `json:"productVersion"`
	BuildTime       string                  `json:"buildTime"`
	Architecture    string                  `json:"architecture"`
	BindAddress     string                  `json:"bindAddress"`
	Port            int                     `json:"port"`
	DatabasePath    string                  `json:"databasePath"`
	BackupDir       string                  `json:"backupDir"`
	NetworkProxy    NetworkProxyStatus      `json:"networkProxy"`
	Scheduler       SchedulerStatus         `json:"scheduler"`
	LastDiagnostics SystemStatusDiagnostics `json:"lastDiagnostics"`
	Summary         DashboardSummary        `json:"summary"`
}

type SystemStatusDiagnostics struct {
	Overall     string `json:"overall"`
	GeneratedAt string `json:"generatedAt"`
	ItemCount   int    `json:"itemCount"`
}

type HealthStatus struct {
	Status      string        `json:"status"`
	GeneratedAt string        `json:"generatedAt"`
	Checks      []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ActionCenter struct {
	GeneratedAt string       `json:"generatedAt"`
	Overall     string       `json:"overall"`
	Items       []ActionItem `json:"items"`
}

type ActionItem struct {
	ID          string   `json:"id"`
	Priority    int      `json:"priority"`
	Level       string   `json:"level"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Count       int      `json:"count"`
	Target      string   `json:"target"`
	Filter      string   `json:"filter,omitempty"`
	Action      string   `json:"action"`
	Samples     []string `json:"samples,omitempty"`
}

type CheckinStatus struct {
	GeneratedAt       string                `json:"generatedAt"`
	Running           bool                  `json:"running"`
	Mode              string                `json:"mode"`
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

type CheckinTodaySummary struct {
	TotalLogs        int `json:"totalLogs"`
	SuccessCount     int `json:"successCount"`
	AlreadyCount     int `json:"alreadyCount"`
	FailedCount      int `json:"failedCount"`
	UnsupportedCount int `json:"unsupportedCount"`
	AuthExpiredCount int `json:"authExpiredCount"`
	DueAccounts      int `json:"dueAccounts"`
}

type CheckinScheduleStatus struct {
	Enabled             bool   `json:"enabled"`
	Time                string `json:"time"`
	RandomDelayMin      int    `json:"randomDelayMin"`
	RandomDelayMax      int    `json:"randomDelayMax"`
	NextRunAt           string `json:"nextRunAt,omitempty"`
	NextWindowStartAt   string `json:"nextWindowStartAt,omitempty"`
	NextWindowEndAt     string `json:"nextWindowEndAt,omitempty"`
	NextRunInSeconds    int64  `json:"nextRunInSeconds"`
	NextWindowInSeconds int64  `json:"nextWindowInSeconds"`
	Message             string `json:"message,omitempty"`
}

type SystemSetting struct {
	Key       string `json:"key"`
	ValueJSON string `json:"valueJson"`
	UpdatedAt string `json:"updatedAt"`
}

type SystemBackup struct {
	FileName  string `json:"fileName"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

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

type DiagnosticItem struct {
	ID            string   `json:"id"`
	Level         string   `json:"level"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Action        string   `json:"action,omitempty"`
	SolutionSteps []string `json:"solutionSteps,omitempty"`
	Count         int      `json:"count,omitempty"`
}

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

type SiteDetail struct {
	Site             UpstreamSite      `json:"site"`
	Detection        UpstreamDetection `json:"detection"`
	Accounts         []ChannelAccount  `json:"accounts"`
	BalanceSnapshots []BalanceSnapshot `json:"balanceSnapshots"`
	CheckinLogs      []CheckinLog      `json:"checkinLogs"`
	Suggestions      []string          `json:"suggestions"`
}

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
	APIKeyStatus         string   `json:"apiKeyStatus,omitempty"`
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
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
}

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

type Notification struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}

type NotificationChannelStatus struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	ConfigValid bool     `json:"configValid"`
	Levels      []string `json:"levels,omitempty"`
}
