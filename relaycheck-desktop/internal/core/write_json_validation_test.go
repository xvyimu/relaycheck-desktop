package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestBulkWriteHandlersRejectMalformedJSON(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	tests := []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "password login", path: "/api/accounts/bulk-password-login", handler: app.handleBulkPasswordLogin},
		{name: "open browser login", path: "/api/accounts/bulk-open-browser-login", handler: app.handleBulkOpenBrowserLogin},
		{name: "finish browser login", path: "/api/accounts/bulk-finish-browser-login", handler: app.handleBulkFinishBrowserLogin},
		{name: "test api keys", path: "/api/accounts/bulk-test-api-keys", handler: app.handleBulkTestAPIKeys},
		{name: "refresh balances", path: "/api/accounts/bulk-refresh-balances", handler: app.handleBulkRefreshBalances},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{"limit":`))
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDeleteUnsupportedCheckinAccountsRejectsMalformedJSONWithoutDeleting(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	siteID := newID()
	accountID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, supports_checkin, created_at, updated_at)
		VALUES (?, 'Unsupported', 'https://unsupported.example', 'newapi', 'healthy', 0, ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO channel_accounts (id, upstream_site_id, display_name, auth_type, login_status, created_at, updated_at)
		VALUES (?, ?, 'Must Keep', 'cookie', 'valid', ?, ?)
	`, accountID, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/delete-unsupported-checkins", strings.NewReader(`{"dryRun":`))
	rec := httptest.NewRecorder()
	app.handleDeleteUnsupportedCheckinAccounts(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var remaining int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE id=?`, accountID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("malformed JSON must not delete accounts; remaining=%d", remaining)
	}
}

func TestSystemProxyTestRejectsMalformedJSONBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	app := &App{
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})},
		networkProxy: NewNetworkProxyStore(defaultNetworkProxyConfig()),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/system/proxy-test", strings.NewReader(`{"targetUrl":`))
	rec := httptest.NewRecorder()
	app.handleSystemProxyTest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if requests.Load() != 0 {
		t.Fatalf("malformed JSON must not start a proxy probe; requests=%d", requests.Load())
	}
}

func TestRemainingOptionalJSONHandlersRejectMalformedJSON(t *testing.T) {
	app := newTestApp(t)
	defer app.Close()

	tests := []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "channel model sync", path: "/api/channels/models/sync", handler: app.handleChannelModelsSync},
		{name: "site detection", path: "/api/upstream-sites/bulk-detect", handler: app.handleBulkDetectUpstreamSites},
		{name: "local newapi sync", path: "/api/local-newapi/missing/sync", handler: func(w http.ResponseWriter, r *http.Request) { app.syncLocalNewAPIInstance(w, r, "missing") }},
		{name: "local newapi preview", path: "/api/local-newapi/missing/sync-preview", handler: func(w http.ResponseWriter, r *http.Request) { app.previewLocalNewAPIInstanceSync(w, r, "missing") }},
		{name: "local newapi mark missing", path: "/api/local-newapi/missing/mark-missing", handler: func(w http.ResponseWriter, r *http.Request) { app.markMissingLocalNewAPIInstance(w, r, "missing") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`{"limit":`))
			rec := httptest.NewRecorder()

			tt.handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			var response apiResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error != "请求参数无效。" {
				t.Fatalf("error = %q, want malformed JSON error", response.Error)
			}
		})
	}
}

func TestDecodeOptionalJSONRejectsTrailingAndUnknownData(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "empty body remains optional", body: "", want: false},
		{name: "trailing JSON value", body: `{"limit":1}{}`, want: true},
		{name: "trailing garbage", body: `{"limit":1} trailing`, want: true},
		{name: "unknown field", body: `{"limit":1,"unexpected":true}`, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input struct {
				Limit int `json:"limit"`
			}
			err := decodeOptionalJSON(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body)), &input)
			if (err != nil) != tt.want {
				t.Fatalf("decodeOptionalJSON() error = %v, want error=%v", err, tt.want)
			}
		})
	}
}

func TestDecodeJSONRejectsTrailingAndUnknownData(t *testing.T) {
	for _, body := range []string{
		`{"limit":1}{}`,
		`{"limit":1} trailing`,
		`{"limit":1,"unexpected":true}`,
	} {
		var input struct {
			Limit int `json:"limit"`
		}
		if err := decodeJSON(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), &input); err == nil {
			t.Fatalf("decodeJSON() accepted invalid request body %q", body)
		}
	}
}
