package core

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type BrowserLoginService struct {
	db              *sql.DB
	dataDir         string
	sessions        *BrowserSessionStore
	loadAuth        func(context.Context, string) (accountAuthContext, error)
	encrypt         func(string) (string, error)
	chromeProxyArgs func() []string
	notify          func(kind, level, title, content, relatedType, relatedID string)
	audit           func(action, level, actor, resourceType, resourceID, summary string, metadata map[string]interface{})
}

func NewBrowserLoginService(app *App) *BrowserLoginService {
	return &BrowserLoginService{
		db:              app.db,
		dataDir:         app.dataDir,
		sessions:        app.browserSessions,
		loadAuth:        app.loadAccountAuth,
		encrypt:         app.encryptText,
		chromeProxyArgs: app.chromeProxyArgs,
		notify:          app.notify,
		audit:           app.audit,
	}
}

func (s *BrowserLoginService) Open(ctx context.Context, id string, auth *accountAuthContext) browserLoginOpenResult {
	if auth == nil {
		loaded, err := s.loadAuth(ctx, id)
		if err != nil {
			return browserLoginOpenResult{Status: "failed", Message: publicAccountFailure("browser_open_load_auth", id, "加载账号授权失败。", err)}
		}
		auth = &loaded
	}
	accountName := auth.AccountName
	siteName := auth.UpstreamSite
	baseURL := auth.BaseURL
	profilePath := auth.BrowserProfilePath
	loginTarget := firstNonEmpty(auth.BrowserLoginURL, auth.LoginPath)
	targetURL := s.ResolveTarget(baseURL, loginTarget, auth.BrowserLoginSource)
	loginURLSource := auth.BrowserLoginSource
	if loginURLSource == "" {
		if auth.BrowserLoginURL != "" {
			loginURLSource = "stored"
		} else {
			loginURLSource = "fallback"
		}
	}
	loginURLConfidence := auth.BrowserLoginConfidence
	if loginURLConfidence == 0 && loginURLSource == "fallback" {
		loginURLConfidence = 0.4
	}
	loginURLReason := firstNonEmpty(auth.BrowserLoginReason, "Resolved browser login URL")
	result := browserLoginOpenResult{
		AccountID:          id,
		AccountName:        accountName,
		SiteName:           siteName,
		URL:                targetURL,
		LoginURLSource:     loginURLSource,
		LoginURLConfidence: loginURLConfidence,
		LoginURLReason:     loginURLReason,
	}

	if session, ok := s.sessions.Get(id); ok {
		result.Status = "already_open"
		result.Message = "该账号网页登录窗口已经打开。"
		result.DebugPort = session.Port
		result.ProfilePath = profilePath
		return result
	}
	usedPorts := map[int]bool{}
	for _, session := range s.sessions.List() {
		usedPorts[session.Port] = true
	}

	if profilePath == "" {
		profilePath = filepath.Join(s.dataDir, "browser-profiles", id)
	}
	if err := os.MkdirAll(profilePath, 0o700); err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_open_create_profile", id, "创建浏览器授权目录失败。", err)
		return result
	}

	port, err := freeDebugPort(usedPorts)
	if err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_open_allocate_port", id, "无法分配浏览器调试端口。", err)
		return result
	}
	chromePath, err := findChrome()
	if err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_open_find_chrome", id, "未找到可用的 Chrome 浏览器。", err)
		return result
	}

	chromeArgs := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + profilePath,
		"--no-first-run",
		"--no-default-browser-check",
		targetURL,
	}
	if proxyArgs := s.chromeProxyArgs(); len(proxyArgs) > 0 {
		chromeArgs = append(chromeArgs[:len(chromeArgs)-1], append(proxyArgs, targetURL)...)
	}
	cmd := exec.Command(chromePath, chromeArgs...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = hiddenProcessAttr()
	}
	if err := cmd.Start(); err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_open_start_chrome", id, "启动 Chrome 失败。", err)
		return result
	}

	s.sessions.Set(id, BrowserLoginSession{AccountID: id, Port: port, StartedAt: time.Now(), PID: cmd.Process.Pid})

	// Watchdog: clean up the session entry when the Chrome process exits so
	// the in-memory map doesn't leak entries for crashed or user-closed
	// browser windows.
	go func(accountID string, proc *os.Process) {
		_, _ = proc.Wait()
		s.sessions.DeleteIfPIDMatches(accountID, proc.Pid)
	}(id, cmd.Process)

	if _, execErr := s.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET auth_type='browser_profile', browser_profile_path=?, login_status='manual_required', updated_at=?
		WHERE id=?
	`, profilePath, now(), id); execErr != nil {
		log.Printf("[accounts] browser login profile path update failed for account %s: %v", id, execErr)
	}
	s.audit("browser_auth.opened", "info", "", "account", id, "网页登录授权窗口已打开。", map[string]interface{}{"accountName": accountName, "siteName": siteName})

	result.Status = "opened"
	result.Message = "网页登录窗口已打开，请完成登录后保存授权。"
	result.URL = targetURL
	result.DebugPort = port
	result.ProfilePath = profilePath
	return result
}

func (s *BrowserLoginService) Save(ctx context.Context, id string, auth *accountAuthContext) browserLoginSaveResult {
	if auth == nil {
		loaded, err := s.loadAuth(ctx, id)
		if err != nil {
			return browserLoginSaveResult{Status: "failed", Message: publicAccountFailure("browser_save_load_auth", id, "加载账号授权失败。", err)}
		}
		auth = &loaded
	}
	result := browserLoginSaveResult{AccountID: id, AccountName: auth.AccountName, SiteName: auth.UpstreamSite}

	session, ok := s.sessions.Get(id)
	if !ok {
		result.Status = "missing"
		result.Message = "没有正在进行的网页登录会话，请先点击网页登录。"
		return result
	}

	cookies, userAgent, err := readChromeSession(session.Port)
	if err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_save_read_session", id, "读取浏览器授权失败，请确认登录已完成。", err)
		return result
	}
	if len(cookies) == 0 {
		result.Status = "failed"
		result.Message = "未检测到 Cookie，请先在浏览器中完成登录。"
		return result
	}

	cookieHeader := buildCookieHeader(cookies)
	encryptedCookie, err := s.encrypt(cookieHeader)
	if err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_save_encrypt", id, "保存浏览器授权失败，请重试。", err)
		return result
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET cookie_encrypted=?, user_agent=?, login_status='valid', last_login_at=?, last_validated_at=?, cookie_expiry_at=?, updated_at=?
		WHERE id=?
	`, encryptedCookie, userAgent, now(), now(), estimateCookieExpiry(), now(), id)
	if err != nil {
		result.Status = "failed"
		result.Message = publicAccountFailure("browser_save_database", id, "保存浏览器授权失败，请重试。", err)
		return result
	}

	s.sessions.Delete(id)
	s.notify("browser_login_saved", "success", "网页登录态已保存", fmt.Sprintf("%s 已保存 %d 个 Cookie。", firstNonEmpty(auth.AccountName, id), len(cookies)), "account", id)
	s.audit("browser_auth.connected", "info", "", "account", id, "网页登录授权已保存。", map[string]interface{}{"accountName": auth.AccountName, "siteName": auth.UpstreamSite, "cookieCount": len(cookies)})

	result.Status = "saved"
	result.Message = "网页登录态已保存。"
	result.CookieCount = len(cookies)
	result.CookiePreview = maskSecret(cookieHeader)
	return result
}

func (s *BrowserLoginService) ResolveTarget(baseURL string, loginURL string, source string) string {
	if strings.EqualFold(source, "manual") {
		return resolveManualBrowserLoginTargetURL(baseURL, loginURL)
	}
	return resolveBrowserLoginTargetURL(baseURL, loginURL)
}

func resolveBrowserLoginTargetURL(baseURL string, loginURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	loginURL = strings.TrimSpace(loginURL)
	if loginURL == "" {
		loginURL = "/login"
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil || base.Scheme == "" || base.Host == "" {
		return loginURL
	}
	parsed, err := url.Parse(loginURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(loginURL, "/")
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return base.ResolveReference(&url.URL{Path: "/login"}).String()
	}
	if !strings.EqualFold(resolved.Host, base.Host) {
		return base.ResolveReference(&url.URL{Path: "/login"}).String()
	}
	return resolved.String()
}

func resolveManualBrowserLoginTargetURL(baseURL string, loginURL string) string {
	loginURL = strings.TrimSpace(loginURL)
	if strings.HasPrefix(loginURL, "http://") || strings.HasPrefix(loginURL, "https://") {
		return loginURL
	}
	return resolveBrowserLoginTargetURL(baseURL, loginURL)
}
