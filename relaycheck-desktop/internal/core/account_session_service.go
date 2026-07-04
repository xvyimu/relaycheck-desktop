package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"relaycheck-desktop/internal/capabilities"
)

type AccountSessionService struct {
	db          *sql.DB
	encrypt     func(string) (string, error)
	doLoginHTTP func(*http.Request, *cookiejar.Jar) (*http.Response, error)
}

func NewAccountSessionService(app *App) *AccountSessionService {
	return &AccountSessionService{
		db:          app.db,
		encrypt:     app.encryptText,
		doLoginHTTP: app.doLoginHTTP,
	}
}

func (s *AccountSessionService) Ensure(ctx context.Context, auth *accountAuthContext) error {
	if auth.Cookie != "" || auth.APIKey != "" || (auth.AccessToken != "" && auth.AuthUserID != "") {
		return nil
	}
	if auth.LoginName == "" || auth.Password == "" {
		return errorsText("没有可用的 Cookie、Token 或账号密码。")
	}
	return s.LoginWithPassword(ctx, auth)
}

func (s *AccountSessionService) LoginWithPassword(ctx context.Context, auth *accountAuthContext) error {
	payloads := []map[string]string{
		{"username": auth.LoginName, "password": auth.Password},
		{"email": auth.LoginName, "password": auth.Password},
		{"account": auth.LoginName, "password": auth.Password},
	}
	loginPaths := capabilities.LoginAPIPaths(auth.LoginPath)
	var lastErr error
	pathFailures := []string{}
	for _, loginPath := range loginPaths {
		var pathErr error
		for _, payload := range payloads {
			body, _ := json.Marshal(payload)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeBaseURL(auth.BaseURL)+loginPath, bytes.NewReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("content-type", "application/json")
			req.Header.Set("accept", "application/json, text/plain, */*")
			req.Header.Set("user-agent", firstNonEmpty(auth.UserAgent, "RelayCheck-Desktop/0.1"))
			jar, _ := cookiejar.New(nil)
			resp, err := s.doLoginHTTP(req, jar)
			if err != nil {
				lastErr = err
				pathErr = err
				continue
			}
			content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			cookies := cookiesFromLoginResponse(resp, jar, req.URL)
			_ = resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("%s HTTP %d: %s", loginPath, resp.StatusCode, firstNonEmpty(extractMessage(string(content)), maskResponse(string(content))))
				pathErr = fmt.Errorf("HTTP 状态码 %d：%s", resp.StatusCode, firstNonEmpty(extractMessage(string(content)), maskResponse(string(content))))
				continue
			}
			if responseExplicitlyFailed(string(content)) {
				lastErr = errorsText(firstNonEmpty(extractMessage(string(content)), "登录失败。"))
				pathErr = lastErr
				continue
			}
			accessToken := extractToken(string(content))
			authUserID := extractUserID(string(content))
			if cookies == "" && accessToken == "" {
				lastErr = fmt.Errorf("%s 未返回 Cookie 或 Token", loginPath)
				pathErr = errorsText("未返回 Cookie 或 Token")
				continue
			}
			if err := s.Save(ctx, auth, cookies, accessToken, authUserID); err != nil {
				return err
			}
			return nil
		}
		if pathErr != nil {
			pathFailures = append(pathFailures, formatLoginPathFailure(loginPath, pathErr))
		}
	}
	if len(pathFailures) > 0 {
		return loginFailuresError(pathFailures)
	}
	if lastErr != nil {
		return lastErr
	}
	return errorsText("登录接口不可用。")
}

func (s *AccountSessionService) Save(ctx context.Context, auth *accountAuthContext, cookie string, accessToken string, authUserID string) error {
	cookieEncrypted, err := s.encrypt(cookie)
	if err != nil {
		return err
	}
	accessEncrypted, err := s.encrypt(accessToken)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE channel_accounts
		SET cookie_encrypted=CASE WHEN ? <> '' THEN ? ELSE cookie_encrypted END,
		    access_token_encrypted=CASE WHEN ? <> '' THEN ? ELSE access_token_encrypted END,
		    auth_user_id=CASE WHEN ? <> '' THEN ? ELSE auth_user_id END,
		    login_status='valid',
		    last_login_at=?,
		    last_validated_at=?,
		    updated_at=?
		WHERE id=?
	`, cookie, cookieEncrypted, accessToken, accessEncrypted, authUserID, authUserID, now(), now(), now(), auth.AccountID)
	if err != nil {
		return err
	}
	if cookie != "" {
		auth.Cookie = cookie
	}
	if accessToken != "" {
		auth.AccessToken = accessToken
	}
	if authUserID != "" {
		auth.AuthUserID = authUserID
	}
	return nil
}

func formatLoginPathFailure(path string, err error) string {
	message := "未知错误"
	if err != nil {
		message = err.Error()
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 120 {
		message = message[:120] + "..."
	}
	return fmt.Sprintf("%s %s", path, message)
}

func loginFailuresError(pathFailures []string) error {
	return fmt.Errorf("登录接口全部失败：%s；建议在账号卡片修正站点登录地址，或改用网页登录授权保存会话。", strings.Join(pathFailures, "；"))
}

func responseExplicitlyFailed(body string) bool {
	var payload map[string]interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return false
	}
	for _, key := range []string{"success", "ok"} {
		if value, exists := payload[key]; exists {
			if boolValue, ok := value.(bool); ok {
				return !boolValue
			}
		}
	}
	if value, exists := payload["code"]; exists {
		if code, ok := toFloat(value); ok {
			return code != 0
		}
	}
	return false
}

func cookiesToHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func cookiesFromLoginResponse(resp *http.Response, jar *cookiejar.Jar, loginURL *url.URL) string {
	if jar != nil && loginURL != nil {
		if cookies := cookiesToHeader(jar.Cookies(loginURL)); cookies != "" {
			return cookies
		}
	}
	if jar != nil && resp != nil && resp.Request != nil && resp.Request.URL != nil {
		if cookies := cookiesToHeader(jar.Cookies(resp.Request.URL)); cookies != "" {
			return cookies
		}
	}
	if resp == nil {
		return ""
	}
	return cookiesToHeader(resp.Cookies())
}

func extractToken(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	for _, key := range []string{"access_token", "accessToken", "token", "session_token"} {
		if value := findString(payload, key); value != "" {
			return value
		}
	}
	return ""
}

func extractUserID(body string) string {
	var payload interface{}
	if json.Unmarshal([]byte(body), &payload) != nil {
		return ""
	}
	if root, ok := payload.(map[string]interface{}); ok {
		if data, ok := root["data"]; ok {
			if found := findNumber(data, "id", "user_id", "userId"); found != nil {
				return fmt.Sprintf("%.0f", *found)
			}
		}
	}
	for _, key := range []string{"user_id", "userId", "id"} {
		if found := findNumber(payload, key); found != nil {
			return fmt.Sprintf("%.0f", *found)
		}
	}
	return ""
}
