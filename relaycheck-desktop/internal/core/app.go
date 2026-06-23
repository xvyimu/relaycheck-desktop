package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type App struct {
	db                 *sql.DB
	dataDir            string
	key                []byte
	sessions           map[string]string
	browserSessions    map[string]BrowserLoginSession
	mu                 sync.RWMutex
	readCache          map[string]readCacheEntry
	readCacheMu        sync.RWMutex
	client             *http.Client
	networkProxy       NetworkProxyConfig
	notificationConfig notificationChannelsConfig
	checkinRun         checkinRunState
	localSyncRun       syncJobRunState
	schedulerCancel    context.CancelFunc
	schedulerStartedAt time.Time
	schedulerWG        sync.WaitGroup
	digestCancel       context.CancelFunc
	digestWG           sync.WaitGroup
	digestChannels     map[string]*webhookChannel
	channelRateLimits  map[string]*channelRateLimiter
	bind               string
	port               int
	allowLocalOutbound bool
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

	dbPath := filepath.Join(dataDir, "relaycheck.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-20000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	app := &App{
		db:              db,
		dataDir:         dataDir,
		key:             key,
		sessions:        map[string]string{},
		browserSessions: map[string]BrowserLoginSession{},
		readCache:       map[string]readCacheEntry{},
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		networkProxy:      defaultNetworkProxyConfig(),
		digestChannels:    map[string]*webhookChannel{},
		channelRateLimits: map[string]*channelRateLimiter{},
		bind:              "127.0.0.1",
		port:              3001,
	}

	if err := app.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.ensureDefaultAdmin(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := app.ensureDefaultSettings(context.Background()); err != nil {
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

func (a *App) Close() error {
	a.mu.Lock()
	cancel := a.schedulerCancel
	a.schedulerCancel = nil
	digestCancel := a.digestCancel
	a.digestCancel = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.schedulerWG.Wait()
	}
	if digestCancel != nil {
		digestCancel()
		a.digestWG.Wait()
	}
	_, _ = a.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return a.db.Close()
}

// DataDir returns the application data directory path.
func (a *App) DataDir() string { return a.dataDir }

func (a *App) SetRuntimeAddress(bind string, port int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bind = bind
	a.port = port
}

func (a *App) ensureDefaultAdmin(ctx context.Context) error {
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app_users WHERE username = ?`, "admin").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password, err := a.bootstrapAdminPassword()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO app_users (id, username, password_hash, display_name, role, must_change_pass, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, newID(), "admin", string(hash), "本地管理员", "admin", 1, now(), now())
	return err
}

func (a *App) bootstrapAdminPassword() (string, error) {
	if password := strings.TrimSpace(os.Getenv("RELAYCHECK_BOOTSTRAP_PASSWORD")); password != "" {
		return password, nil
	}

	path := filepath.Join(a.dataDir, "bootstrap-admin-password.txt")
	if data, err := os.ReadFile(path); err == nil {
		if password := strings.TrimSpace(string(data)); password != "" {
			return password, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	password := randomToken()
	if err := os.WriteFile(path, []byte(password+"\n"), 0o600); err != nil {
		return "", err
	}
	return password, nil
}

func (a *App) ensureDefaultSettings(ctx context.Context) error {
	defaults := map[string]string{
		"app.general":           `{"productName":"RelayCheck Desktop","bindAddress":"127.0.0.1","port":3001,"lightweightMode":true}`,
		"scanner.targets":       `{"hosts":["127.0.0.1","localhost"],"ports":[3000,3001,8080,9999,3010],"allowCustomUrls":true}`,
		"checkin.schedule":      `{"enabled":true,"time":"08:00","randomDelayMinutes":[0,120],"siteConcurrency":1,"globalConcurrency":3,"siteMinIntervalSeconds":2}`,
		"network.proxy":         `{"enabled":false,"url":"http://127.0.0.1:7897","bypassLocal":true}`,
		"sync.schedule":         `{"enabled":true,"intervalMinutes":30,"mode":"local-newapi","runOnStartup":false}`,
		"notification.channels": `{"enabled":false,"defaultLevels":["warning","error"],"channels":[{"type":"webhook","name":"默认 Webhook","enabled":false,"config":{"url":"","hmacSecret":"","mode":"all","timeoutSeconds":10},"levels":["warning","error"],"types":["scheduled_checkin_failed","scheduled_sync_failed"]},{"type":"telegram","name":"Telegram Bot","enabled":false,"config":{"botToken":"","chatId":"","mode":"failure"},"levels":["warning","error"],"types":["scheduled_checkin_failed"]},{"type":"bark","name":"Bark","enabled":false,"config":{"url":"","mode":"failure","group":"RelayCheck"},"levels":["warning","error"]},{"type":"serverchan","name":"ServerChan","enabled":false,"config":{"sendKey":"","mode":"failure"},"levels":["warning","error"]},{"type":"email","name":"SMTP 邮件","enabled":false,"config":{"smtpHost":"","smtpPort":587,"smtpTls":true,"username":"","password":"","fromAddr":"","toAddr":"","mode":"failure"},"levels":["warning","error"]}]}`,
	}
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

func (a *App) withSession(r *http.Request) (string, error) {
	cookie, err := r.Cookie("relaycheck_session")
	if err != nil || cookie.Value == "" {
		return "", errors.New("not authenticated")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	userID := a.sessions[cookie.Value]
	if userID == "" {
		return "", errors.New("not authenticated")
	}
	return userID, nil
}

func (a *App) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := a.withSession(r); err != nil {
			writeError(w, http.StatusUnauthorized, "未登录或会话已过期")
			return
		}
		next(w, r)
	}
}
