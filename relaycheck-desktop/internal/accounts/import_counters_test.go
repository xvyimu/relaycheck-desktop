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

func TestSkippedExcludedSamplesAndMatchedToken(t *testing.T) {
	db := setupAccountsTestDB(t)
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": "1", "name": "Good", "base_url": "https://good.example", "status": "1"},
				map[string]interface{}{"id": "2", "name": "9router free", "base_url": "https://x.example", "status": "1"},
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
		context.Background(), "https://newapi.example", "tok", "1", "Sample Inst",
		false, true, false, 100, false,
	)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if intFromResult(result, "skippedExcluded") != 1 {
		t.Fatalf("skippedExcluded = %d", intFromResult(result, "skippedExcluded"))
	}
	samples, ok := result["skippedExcludedSamples"].([]ExcludedChannelSample)
	if !ok || len(samples) != 1 {
		t.Fatalf("samples type/len wrong: %#v", result["skippedExcludedSamples"])
	}
	if samples[0].MatchedToken != "9router" {
		t.Fatalf("MatchedToken = %q", samples[0].MatchedToken)
	}
	if samples[0].SourceChannelID != "2" {
		t.Fatalf("SourceChannelID = %q", samples[0].SourceChannelID)
	}
	raw, _ := json.Marshal(samples[0])
	if bytes.Contains(raw, []byte("\"tok\"")) || bytes.Contains(bytes.ToLower(raw), []byte("password")) {
		t.Fatalf("sample JSON looks secret-bearing: %s", raw)
	}
}

func TestListExcludedRelaySiteRules(t *testing.T) {
	rules := ListExcludedRelaySiteRules()
	if len(rules) == 0 {
		t.Fatal("expected rules")
	}
	for _, r := range rules {
		if r.Token == "" || r.Description == "" {
			t.Fatalf("empty rule: %+v", r)
		}
	}
}

func TestFormatAndSaveLastSyncSummary(t *testing.T) {
	db := setupAccountsTestDB(t)
	infra := &stubInfra{db: db}
	svc := NewService(infra)
	now := infra.Now()
	_, err := db.Exec(`INSERT INTO local_newapi_instances (id, name, base_url, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, "inst-sync", "S", "https://s.example", "active", now, now)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	result := map[string]interface{}{
		"fetchedCount":     3,
		"importedCount":    2,
		"skippedExcluded":  1,
		"skippedNoBaseURL": 0,
		"sitesCreated":     1,
		"sitesMerged":      0,
	}
	summary := FormatLastSyncSummary(result)
	if summary == "" || !bytes.Contains([]byte(summary), []byte("拉取")) {
		t.Fatalf("summary empty/unexpected: %q", summary)
	}
	if err := svc.SaveLocalNewAPILastSyncSummary(context.Background(), "inst-sync", result); err != nil {
		t.Fatalf("save: %v", err)
	}
	inst, err := svc.GetLocalNewAPIInstance(context.Background(), "inst-sync")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if inst.LastSyncAt == "" || inst.LastSyncSummary == "" {
		t.Fatalf("last sync not persisted: at=%q summary=%q", inst.LastSyncAt, inst.LastSyncSummary)
	}
	items, err := svc.ListLocalNewAPIInstances(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v len=%d", err, len(items))
	}
	if items[0].LastSyncSummary == "" {
		t.Fatal("list missing LastSyncSummary")
	}
}
