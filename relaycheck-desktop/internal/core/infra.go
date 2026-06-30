package core

import (
	"database/sql"
	"net/http"
	"sync"
)

// SharedInfra exposes shared infrastructure to domain packages. Domain
// packages depend on this interface rather than on *App directly, so they can
// be tested in isolation and migrated to alternative implementations (e.g.
// cloud backends) without touching business logic.
//
// The interface starts minimal in Phase 0 and grows as each domain is
// extracted: each domain's "interface seam" commit adds the relevant port
// interface (e.g. NotificationHubPort, TaskRunnerPort) to this interface and
// makes *App satisfy it. Keeping the ports co-located with the domain that
// consumes them avoids premature abstraction and keeps Phase 0 a small,
// low-risk change.
type SharedInfra interface {
	// DB returns the application's SQLite database handle.
	DB() *sql.DB
	// HTTPClient returns the shared HTTP client (with proxy and timeout
	// configured).
	HTTPClient() *http.Client
	// Key returns the instance encryption key used for credential fields.
	Key() []byte
	// DataDir returns the application data directory path.
	DataDir() string
	// Locker returns the coarse-grained mutex protecting App state that
	// domains need to coordinate on (e.g. browserSessions, digestChannels).
	// Domains that need finer-grained locking should introduce their own
	// mutex rather than relying on this one.
	Locker() sync.Locker
}

// Compile-time assertion that *App implements SharedInfra. If App stops
// satisfying the interface (e.g. a getter is removed), this fails at build
// time rather than at the first call site.
var _ SharedInfra = (*App)(nil)

// DB returns the application's SQLite database handle.
func (a *App) DB() *sql.DB { return a.db }

// HTTPClient returns the shared HTTP client.
func (a *App) HTTPClient() *http.Client { return a.client }

// Key returns the instance encryption key.
func (a *App) Key() []byte { return a.key }

// Locker returns the coarse-grained App mutex as a sync.Locker. Callers that
// only need read access can cast to sync.RWMutex via a.mu directly, but new
// domain code should prefer its own locking strategy.
func (a *App) Locker() sync.Locker { return &a.mu }
