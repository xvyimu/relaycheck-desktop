package core

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"relaycheck-desktop/internal/capabilities"
)

type AccountAPIClient struct {
	doHTTP            func(*http.Request) (*http.Response, error)
	doHTTPWithTimeout func(*http.Request, time.Duration) (*http.Response, error)
	externalURLPolicy func() outboundURLPolicy
}

func NewAccountAPIClient(app *App) *AccountAPIClient {
	return &AccountAPIClient{
		doHTTP:            app.doHTTP,
		doHTTPWithTimeout: app.doHTTPWithTimeout,
		externalURLPolicy: app.externalURLPolicy,
	}
}

func (c *AccountAPIClient) Do(ctx context.Context, auth accountAuthContext, method string, path string, body []byte) (int, string, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, normalizeBaseURL(auth.BaseURL)+path, reader)
	if err != nil {
		return 0, "", err
	}
	applyAccountAPIHeaders(req, auth, body != nil)
	resp, err := c.doHTTP(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return resp.StatusCode, string(content), nil
}

func (c *AccountAPIClient) DoWithTimeout(ctx context.Context, auth accountAuthContext, method string, path string, body []byte, timeout time.Duration) (int, string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	baseURL, err := safeNormalizeBaseURL(requestCtx, auth.BaseURL, c.externalURLPolicy())
	if err != nil {
		return 0, "", err
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(requestCtx, method, baseURL+path, reader)
	if err != nil {
		return 0, "", err
	}
	applyAccountAPIKeyHeaders(req, auth, body != nil)
	resp, err := c.doHTTPWithTimeout(req, timeout+time.Second)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	return resp.StatusCode, string(content), nil
}

func applyAccountAPIHeaders(req *http.Request, auth accountAuthContext, hasBody bool) {
	applyAccountAPIBaseHeaders(req, auth, hasBody)
	if auth.Cookie != "" {
		req.Header.Set("cookie", auth.Cookie)
	}
	if auth.AuthUserID != "" {
		headerName := "New-Api-User"
		if h, ok := capabilities.UserIDHeaderForKind(auth.SiteKind); ok {
			headerName = h
		}
		req.Header.Set(headerName, auth.AuthUserID)
	}
	if token := firstNonEmpty(auth.AccessToken, auth.APIKey); token != "" {
		setBearerAuthorization(req, token)
	}
}

func applyAccountAPIKeyHeaders(req *http.Request, auth accountAuthContext, hasBody bool) {
	applyAccountAPIBaseHeaders(req, auth, hasBody)
	if auth.APIKey != "" {
		setBearerAuthorization(req, auth.APIKey)
	}
}

func applyAccountAPIBaseHeaders(req *http.Request, auth accountAuthContext, hasBody bool) {
	req.Header.Set("user-agent", firstNonEmpty(auth.UserAgent, "RelayCheck-Desktop/0.1"))
	req.Header.Set("accept", "application/json, text/plain, */*")
	if hasBody {
		req.Header.Set("content-type", "application/json")
	}
}

func setBearerAuthorization(req *http.Request, token string) {
	if !strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = "Bearer " + token
	}
	req.Header.Set("authorization", token)
}
