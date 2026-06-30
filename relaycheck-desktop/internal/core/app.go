package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"relaycheck-desktop/internal/accounts"
	"relaycheck-desktop/internal/autostart"
	"relaycheck-desktop/internal/backup"
	"relaycheck-desktop/internal/channels"
	"relaycheck-desktop/internal/legacycheck"
	"relaycheck-desktop/internal/notifications"
	"relaycheck-desktop/internal/sites"
	"relaycheck-desktop/internal/versioncheck"

	_ "modernc.org/sqlite"
)

type App struct {
	db                  *sql.DB
	dataDir             string
	key                 []byte
	crypto              *CryptoService
	accountAuth         *AccountAuthRepository
	schedulerRepo       *SchedulerRepo
	browserSessions     *BrowserSessionStore
	mu                  sync.RWMutex
	readCache           *ReadCacheStore
	client              *http.Client
	networkProxy        *NetworkProxyStore
	notificationHub     *notifications.NotificationHub
	backupService       *backup.Service
	sitesService        *sites.Service
	channelsService     *channels.Service
	accountsService     *accounts.Service
	versionCheckService *versioncheck.Service
	legacyCheckService  *legacycheck.Service
	autostartService    *autostart.Service
	checkinRun          *CheckinRunStore
	localSyncRun        *SyncJobRunStore
	channelHealthRun    *SyncJobRunStore
	schedulerCancel     context.CancelFunc
	schedulerStartedAt  time.Time
	schedulerWG         sync.WaitGroup
	taskRunner          *TaskRunner
	bind                string
	port                int
	preferredPort       int
	portConflict        bool
	allowLocalOutbound  bool
}

var fallbackIDCounter atomic.Uint64

type checkinRunState struct {
	Running           bool
	Mode              string
	CurrentAccountID  string
	CurrentAccount    string
	CurrentSite       string
	CurrentMessage    string
	TotalAccounts     int
	ProcessedAccounts int
	SuccessCount      int
	AlreadyCount      int
	FailedCount       int
	UnsupportedCount  int
	AuthExpiredCount  int
	StartedAt         string
	UpdatedAt         string
	FinishedAt        string
	LastRunMessage    string
}

type BrowserLoginSession struct {
	AccountID string
	Port      int
	StartedAt time.Time
	PID       int
}

// NewApp creates a new App instance rooted at the given directory.
func NewApp(root string) (*App, error) {
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "keys"), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "browser-profiles"), 0o700); err != nil {
		return nil, err
	}

	key, err := loadOrCreateKey(filepath.Join(dataDir, "keys", "instance.key"))
	if err != nil {
		return nil, err
	}

	cryptoSvc := NewCryptoService(key)

	dbPath := filepath.Join(dataDir, "relaycheck.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-20000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	accountAuthRepo := NewAccountAuthRepository(db, cryptoSvc)

	app := &App{
		db:              db,
		dataDir:         dataDir,
		key:             key,
		crypto:          cryptoSvc,
		accountAuth:     accountAuthRepo,
		schedulerRepo:   NewSchedulerRepo(db),
		browserSessions: NewBrowserSessionStore(),
		readCache:       NewReadCacheStore(),
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		networkProxy:     NewNetworkProxyStore(defaultNetworkProxyConfig()),
		taskRunner:       newTaskRunner(),
		bind:             "127.0.0.1",
		port:             3001,
		checkinRun:       NewCheckinRunStore(),
		localSyncRun:     NewSyncJobRunStore(),
		channelHealthRun: NewSyncJobRunStore(),
	}

	// Two-phase init: NotificationHTTPPort is satisfied by *App itself
	// (externalURLPolicy + doHTTPWithTimeout), so the hub can only be wired
	// up after the app struct exists. Other repositories (accountAuth) are
	// constructed before app; the hub is the one repo that needs app.
	app.notificationHub = notifications.NewNotificationHub(db, cryptoSvc, app)
	// backup.Service depends on backup.Infra (DB + DatabasePath + BackupsDir
	// + ReopenDatabase + ReloadNotificationConfig + ProductVersion), all of
	// which are satisfied by *App itself, so it is wired up after the app
	// struct exists.
	app.backupService = backup.NewService(app)
	// sites.Service depends on sites.Infra (DB + DoHTTP + ValidateOutboundURL
	// + ValidateLocalURL + AllowLocalOutbound + Notify + Audit + Now + NewID),
	// all of which are satisfied by *App itself, so it is wired up after the
	// app struct exists.
	app.sitesService = sites.NewService(app)
	// channels.Service depends on channels.Infra (DB + DoHTTP +
	// DoHTTPWithTimeout + DecryptText + DetectUpstream +
	// EnsureUpstreamSiteForChannel + Notify + Audit + Now + NewID +
	// InvalidateReadCache + LoadSchedulerRun + LoadCheckinScheduleConfig +
	// SafeNormalizeBaseURL), all of which are satisfied by *App itself, so
	// it is wired up after the app struct exists.
	app.channelsService = channels.NewService(app)
	// accounts.Service depends on accounts.Infra (DB + DoHTTP + EncryptText +
	// DecryptText + DetectUpstreamForImport + EnsureChannelSiteForImport +
	// Notify + Audit + Now + NewID), all of which are satisfied by *App
	// itself, so it is wired up after the app struct exists. The
	// "ForImport"-suffixed adapters in accounts_infra.go are distinct from
	// the channels.Infra adapters (DetectUpstream / EnsureUpstreamSiteForChannel)
	// so a single *App satisfies both interfaces.
	app.accountsService = accounts.NewService(app)
	// versioncheck.Service depends on versioncheck.Infra (DB + HTTPClient +
	// ProductVersion + ValidateOutboundURLStrict), all of which are satisfied
	// by *App itself, so it is wired up after the app struct exists.
	app.versionCheckService = versioncheck.NewService(app)
	// legacycheck.Service depends on legacycheck.Infra (DataDir), which is
	// satisfied by *App itself, so it is wired up after the app struct
	// exists.
	app.legacyCheckService = legacycheck.NewService(app)
	// autostart.Service is purely platform-based (os.Executable + env), so
	// it has no Infra dependency and is wired up statelessly.
	app.autostartService = autostart.NewService()

	if err := app.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.ensureDefaultSettings(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.ensureGlobalScheduleRecord(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.reloadNetworkProxyConfig(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.reloadNotificationConfig(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return app, nil
}

// Close releases resources held by the App, including the database.
func (a *App) Close() error {
	a.mu.Lock()
	cancel := a.schedulerCancel
	a.schedulerCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.schedulerWG.Wait()
	}
	if a.notificationHub != nil {
		a.notificationHub.Close()
	}
	if _, execErr := a.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); execErr != nil {
		log.Printf("[app] wal checkpoint failed: %v", execErr)
	}
	return a.db.Close()
}

// DataDir returns the application data directory path.
func (a *App) DataDir() string { return a.dataDir }

// SetRuntimeAddress configures the bind address and port used for Origin validation.
func (a *App) SetRuntimeAddress(bind string, port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bind = bind
	a.port = port
}

// SetPortConflict records the preferred port and whether it conflicted at startup.
func (a *App) SetPortConflict(preferredPort int, conflict bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.preferredPort = preferredPort
	a.portConflict = conflict
}

func (a *App) ensureDefaultSettings(ctx context.Context) error {
	defaults := map[string]string{
		"app.general":             `{"productName":"RelayCheck Desktop","bindAddress":"127.0.0.1","port":3001,"lightweightMode":true}`,
		"app.version_check_url":   `""`,
		"scanner.targets":         `{"hosts":["127.0.0.1","localhost"],"ports":[3000,3001,8080,9999,3010],"allowCustomUrls":true}`,
		"checkin.schedule":        `{"enabled":true,"time":"08:00","randomDelayMinutes":[0,120],"siteConcurrency":1,"globalConcurrency":3,"siteMinIntervalSeconds":2}`,
		"network.proxy":           `{"enabled":false,"url":"http://127.0.0.1:7897","bypassLocal":true}`,
		"sync.schedule":           `{"enabled":true,"intervalMinutes":30,"mode":"local-newapi","runOnStartup":false}`,
		"channel.health.schedule": `{"enabled":true,"intervalMinutes":60,"runOnStartup":false,"limit":20,"onlyRisky":false}`,
		"notification.channels":   `{"enabled":false,"defaultLevels":["warning","error"],"channels":[{"type":"webhook","name":"默认 Webhook","enabled":false,"config":{"url":"","hmacSecret":"","mode":"all","timeoutSeconds":10,"maxRetries":3},"levels":["warning","error"],"types":["scheduled_checkin_failed","scheduled_sync_failed"]},{"type":"telegram","name":"Telegram Bot","enabled":false,"config":{"botToken":"","chatId":"","mode":"failure"},"levels":["warning","error"],"types":["scheduled_checkin_failed"]},{"type":"bark","name":"Bark","enabled":false,"config":{"url":"","mode":"failure","group":"RelayCheck"},"levels":["warning","error"]},{"type":"serverchan","name":"ServerChan","enabled":false,"config":{"sendKey":"","mode":"failure"},"levels":["warning","error"]},{"type":"email","name":"SMTP 邮件","enabled":false,"config":{"smtpHost":"","smtpPort":587,"smtpTls":true,"username":"","password":"","fromAddr":"","toAddr":"","mode":"failure"},"levels":["warning","error"]},{"type":"desktop","name":"桌面通知","enabled":true,"config":{"mode":"all","sound":true},"levels":["info","warning","error"]}]}`,
	}
	defaults["notification.channels"] = withDefaultHealthNotificationTypes(defaults["notification.channels"])
	for key, value := range defaults {
		_, err := a.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO system_settings (id, key, value_json, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, newID(), key, value, now(), now())
		if err != nil {
			return err
		}
	}
	return nil
}

func newID() string {
	return newIDFromReader(rand.Reader)
}

func newIDFromReader(reader io.Reader) string {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(reader, buf); err == nil {
		return hex.EncodeToString(buf)
	}
	binary.BigEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint64(buf[8:], fallbackIDCounter.Add(1))
	return hex.EncodeToString(buf)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// todayCST returns the current calendar date in China Standard Time (UTC+8).
// Use this for "today" filters on logs/summaries so that the frontend, the
// action center, and the diagnostics endpoint agree on what "today" means
// regardless of the server's local timezone.
func todayCST() string {
	return time.Now().In(cstZone()).Format("2006-01-02")
}

// cstZone returns the China Standard Time (UTC+8) location. Centralising this
// here so that schedulers, summaries, and date filters all share one
// definition instead of each call site reconstructing time.FixedZone.
func cstZone() *time.Location {
	return time.FixedZone("CST", 8*3600)
}

// nowCST returns the current time in CST. Use for scheduler ticks so that
// HH:MM config values like "08:00" are interpreted as 08:00 CST regardless
// of the server's local timezone.
func nowCST() time.Time {
	return time.Now().In(cstZone())
}

// withSession authenticates the incoming request.
//
// RelayCheck Desktop binds to 127.0.0.1 and SecureLocalHandler already
// rejects requests whose Host header does not match a loopback address
// (preventing DNS-rebinding). However, simple cross-site form POSTs can
// still bypass CORS preflight and trigger state-changing endpoints. To
// close that CSRF gap, we require the Origin header (when present) to
// point at the same loopback host:port for any state-changing method.
//
// Non-browser clients (curl, internal scheduler calls) do not send an
// Origin header and are allowed through, relying on the Host check and
// the loopback-only bind for isolation.
func (a *App) withSession(r *http.Request) (string, error) {
	if isStateChangingMethod(r.Method) {
		if err := a.validateOrigin(r); err != nil {
			return "", err
		}
	}
	return "local", nil
}

func (a *App) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := a.withSession(r); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		next(w, r)
	}
}

func isStateChangingMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

func (a *App) validateOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return errors.New("invalid origin")
	}
	if u.Scheme != "http" {
		return errors.New("origin scheme not allowed")
	}
	if !a.allowedHost(u.Host) {
		return errors.New("origin not allowed")
	}
	return nil
}

// DB is already provided by infra.go (SharedInfra interface).

// DoHTTP is the exported adapter for the sites package's Infra interface.
// It delegates to doHTTP so the sites service honours the host's HTTP client
// and proxy configuration.
func (a *App) DoHTTP(req *http.Request) (*http.Response, error) { return a.doHTTP(req) }

// ValidateLocalURL is the exported adapter for the sites package's Infra
// interface. It applies a permissive outbound URL policy that allows
// loopback addresses, used by the sites service when probing local NewAPI
// instances.
func (a *App) ValidateLocalURL(ctx context.Context, raw string) (*url.URL, error) {
	return validateOutboundHTTPURL(ctx, raw, outboundURLPolicy{AllowLocal: true})
}

// AllowLocalOutbound is the exported adapter for the sites package's Infra
// interface. It reports whether the host permits loopback outbound
// connections, controlling the sites service's DetectUpstream URL policy.
func (a *App) AllowLocalOutbound() bool { return a.allowLocalOutbound }

// Notify is the exported adapter for the sites package's Infra interface.
// It delegates to the host's notification hub dispatcher.
func (a *App) Notify(kind, level, title, content, relatedType, relatedID string) {
	a.notify(kind, level, title, content, relatedType, relatedID)
}

// Audit is the exported adapter for the sites package's Infra interface.
// It delegates to the host's audit log recorder.
func (a *App) Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{}) {
	a.audit(action, level, userID, entityType, entityID, detail, metadata)
}

// Now is the exported adapter for the sites package's Infra interface.
// It returns the host's current timestamp string (ISO 8601 UTC).
func (a *App) Now() string { return now() }

// NewID is the exported adapter for the sites package's Infra interface.
// It returns a fresh host-generated identifier.
func (a *App) NewID() string { return newID() }
