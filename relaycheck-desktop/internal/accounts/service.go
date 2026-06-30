package accounts

import (
	"context"
	"database/sql"
	"net/http"
)

// Infra is the subset of the host application that the accounts domain
// depends on. Extracting it breaks the reverse reference from the accounts
// service back to the host god object. The host (e.g. *core.App) satisfies
// this interface by providing database access, HTTP client, encryption,
// upstream detection forwarding, notification/audit hooks, and time/id
// generators.
//
// Method names that would collide with adapters already provided for other
// packages (channels.DetectUpstream, channels.EnsureUpstreamSiteForChannel)
// use distinct "ForImport" suffixes so a single *App can satisfy multiple
// packages' Infra interfaces without Go method-name conflicts.
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DB returns the application's SQLite database handle.
	DB() *sql.DB
	// DoHTTP executes req with the host's default HTTP client (honouring
	// proxy config and test-mode client overrides). Used by
	// FetchAdminAPIChannels.
	DoHTTP(req *http.Request) (*http.Response, error)
	// EncryptText encrypts a plaintext value with the host's crypto service.
	// Used by the import flows to persist channel keys / account passwords.
	EncryptText(plaintext string) (string, error)
	// DecryptText decrypts a ciphertext produced by the host's crypto
	// service. Used by ResolveLocalNewAPISyncToken to recover saved tokens.
	DecryptText(ciphertext string) (string, error)
	// DetectUpstreamForImport probes a remote base URL and returns the
	// detection result as an accounts.Detection mirror. Used by the admin
	// API / SQLite import flows when detectAfterImport is set.
	DetectUpstreamForImport(ctx context.Context, raw string) (Detection, error)
	// EnsureChannelSiteForImport upserts an upstream_sites row for the
	// given channel. Used by the import flows to create sites for imported
	// channels. Returns the site ID and whether a new row was created.
	EnsureChannelSiteForImport(ctx context.Context, channelID, name, rawBaseURL, kind string, detection *Detection) (string, bool, error)
	// Notify dispatches a notification through the host's notification hub.
	Notify(kind, level, title, content, relatedType, relatedID string)
	// Audit records an audit log entry.
	Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{})
	// Now returns the host's current timestamp string (ISO 8601 UTC).
	Now() string
	// NewID returns a fresh host-generated identifier.
	NewID() string
}

// Service implements the accounts-import domain: Chrome password CSV import,
// legacy config_site*.json import, NewAPI admin-API / SQLite channel import,
// local NewAPI instance management, sync-preview diffing, missing-channel
// reconciliation, and auto-detect-and-import.
//
// It owns the local_newapi_instances / imported_channels / channel_accounts
// / upstream_sites table round-tripping for the import flows, while relying
// on Infra for database access, HTTP, encryption, detection forwarding, and
// lifecycle hooks. The host application delegates its *App handler/forwarding
// methods to this Service.
//
// The account CRUD / browser-login / API-key-test logic remains in the host
// (core) package because it is tightly coupled to the checkin run state and
// browser-session store.
type Service struct {
	infra Infra
}

// NewService constructs an accounts Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}
