package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestImportChannelRecordOutcomeCounters(t *testing.T) {
	db := setupAccountsTestDB(t)
	n := 0
	infra := &stubInfra{db: db, newIDFn: func() string {
		n++
		return fmt.Sprintf("id-%04d", n)
	}}
	svc := NewService(infra)

	// Seed instance so import can attach channels.
	now := infra.Now()
	_, err := db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"inst-counters", "Counter Instance", "https://counters.example", "active", now, now)
	if err != nil {
		t.Fatalf("seed instance: %v", err)
	}

	cases := []struct {
		name      string
		record    map[string]interface{}
		wantSkip  importSkipReason
		wantNoURL bool
	}{
		{
			name: "normal",
			record: map[string]interface{}{
				"id":       "ch-1",
				"name":     "Primary Relay",
				"base_url": "https://relay.example",
				"status":   "1",
			},
			wantSkip: importSkipNone,
		},
		{
			name: "excluded",
			record: map[string]interface{}{
				"id":       "ch-2",
				"name":     "9router free",
				"base_url": "https://9router.example",
				"status":   "1",
			},
			wantSkip: importSkipExcluded,
		},
		{
			name: "no base url",
			record: map[string]interface{}{
				"id":     "ch-3",
				"name":   "No URL Channel",
				"status": "1",
			},
			wantSkip:  importSkipNone,
			wantNoURL: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := svc.importChannelRecordOutcome(context.Background(), "inst-counters", tc.record, false, true, false, "admin_api")
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if out.Skip != tc.wantSkip {
				t.Fatalf("Skip = %q, want %q", out.Skip, tc.wantSkip)
			}
			if out.NoBaseURL != tc.wantNoURL {
				t.Fatalf("NoBaseURL = %v, want %v", out.NoBaseURL, tc.wantNoURL)
			}
			if tc.wantSkip == importSkipNone && out.ChannelID == "" {
				t.Fatal("expected channel id for imported row")
			}
			if tc.wantSkip == importSkipExcluded && out.ChannelID != "" {
				t.Fatal("excluded row should not create channel id")
			}
		})
	}
}

func TestImportChannelsFromAdminAPIWithOptionsCounters(t *testing.T) {
	db := setupAccountsTestDB(t)
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": "1", "name": "Good", "base_url": "https://good.example", "status": "1"},
				map[string]interface{}{"id": "2", "name": "tokenrouter site", "base_url": "https://token.example", "status": "1"},
				map[string]interface{}{"id": "3", "name": "No URL", "status": "1"},
			},
		},
	}
	body, _ := json.Marshal(payload)
	n := 0
	infra := &stubInfra{
		db: db,
		newIDFn: func() string {
			n++
			return fmt.Sprintf("id-%04d", n)
		},
		doHTTPFn: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		},
	}
	svc := NewService(infra)

	result, err := svc.ImportChannelsFromAdminAPIWithOptions(
		context.Background(),
		"https://newapi.example",
		"test-token",
		"1",
		"Counter API Instance",
		false,
		true,
		false,
		100,
		false,
	)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	assertInt := func(key string, want int) {
		t.Helper()
		got := intFromResult(result, key)
		if got != want {
			t.Fatalf("%s = %d, want %d (result=%v)", key, got, want, result)
		}
	}
	assertInt("fetchedCount", 3)
	assertInt("importedCount", 2)
	assertInt("skippedExcluded", 1)
	assertInt("skippedNoBaseURL", 1)
	assertInt("sitesCreated", 1)
	assertInt("sitesMerged", 0)
}
