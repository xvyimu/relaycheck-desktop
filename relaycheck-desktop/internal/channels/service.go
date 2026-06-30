package channels

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// GlobalScheduleSiteID mirrors the host's virtual site ID used for the global
// checkin schedule. It must stay in sync with core.channel_schedules and
// sites.globalScheduleSiteID.
const GlobalScheduleSiteID = "__global__"

// SchedulerJobSync mirrors core.schedulerJobSync. Used by NextSyncCalendarItem
// to read the next sync run from the host's scheduler_runs table.
const SchedulerJobSync = "sync.local_newapi"

// Infra is the subset of the host application that the channels domain
// depends on. Extracting it breaks the reverse reference from the channels
// service back to the host god object. The host (e.g. *core.App) satisfies
// this interface by providing database access, HTTP client, decryption,
// upstream detection forwarding, notification/audit hooks, time/id generators,
// scheduler info, and outbound URL validation.
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DB returns the application's SQLite database handle.
	DB() *sql.DB
	// DoHTTP executes req with the host's default HTTP client (honouring
	// proxy config and test-mode client overrides).
	DoHTTP(req *http.Request) (*http.Response, error)
	// DoHTTPWithTimeout executes req with an explicit timeout. Used by
	// FetchChannelModelsWithKey and SyncSitePricing.
	DoHTTPWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error)
	// DecryptText decrypts a ciphertext produced by the host's crypto
	// service. Used by SyncChannelModels to recover channel API keys.
	DecryptText(ciphertext string) (string, error)
	// DetectUpstream probes a remote base URL and returns the detection
	// result. Used by DetectChannel. Implementations should forward to the
	// host's sites service and convert sites.Detection to channels.Detection.
	DetectUpstream(ctx context.Context, raw string) (Detection, error)
	// EnsureUpstreamSiteForChannel upserts an upstream_sites row for the
	// given channel. Used by DetectChannel. Returns the site ID and whether
	// a new row was created.
	EnsureUpstreamSiteForChannel(ctx context.Context, input EnsureSiteInput) (string, bool, error)
	// Notify dispatches a notification through the host's notification hub.
	Notify(kind, level, title, content, relatedType, relatedID string)
	// Audit records an audit log entry.
	Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{})
	// Now returns the host's current timestamp string (ISO 8601 UTC).
	Now() string
	// NewID returns a fresh host-generated identifier.
	NewID() string
	// InvalidateReadCache clears the host's short-lived read cache. Called
	// after mutations so subsequent reads see fresh data.
	InvalidateReadCache()
	// LoadSchedulerRun reads the scheduler_runs row for the given job key.
	// Used by NextSyncCalendarItem to surface upcoming sync runs.
	LoadSchedulerRun(ctx context.Context, jobKey string) (SchedulerRunRecord, error)
	// LoadCheckinScheduleConfig reads the checkin.schedule system setting.
	// Used by EnsureGlobalScheduleRecord and SyncGlobalScheduleRecord.
	LoadCheckinScheduleConfig(ctx context.Context) CheckinScheduleConfig
	// SafeNormalizeBaseURL validates and normalizes raw against the host's
	// outbound URL policy (SSRF defences, controlled by allowLocalOutbound).
	// Used by FetchChannelModelsWithKey and SyncSitePricing.
	SafeNormalizeBaseURL(ctx context.Context, raw string) (string, error)
}

// Service implements the relay-channel domain: channel CRUD, source-sync
// status transitions, upstream detection wiring, per-channel model sync,
// channel health overview, per-site checkin scheduling, and model pricing
// source extraction.
//
// It owns the imported_channels / channel_schedules / site_pricing_cache
// table round-tripping and the model/pricing extraction engine, while
// relying on Infra for database access, HTTP, decryption, URL validation,
// detection forwarding, and lifecycle hooks. The host application delegates
// its *App handler/forwarding methods to this Service.
type Service struct {
	infra Infra
}

// NewService constructs a channels Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}
