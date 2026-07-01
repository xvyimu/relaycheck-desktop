package channels

import (
	"testing"
)

func TestChannelHealthLevelRank(t *testing.T) {
	cases := map[string]int{
		"danger":  3,
		"warning": 2,
		"success": 1,
		"":        0,
		"unknown": 0,
	}
	for level, want := range cases {
		if got := channelHealthLevelRank(level); got != want {
			t.Errorf("channelHealthLevelRank(%q) = %d, want %d", level, got, want)
		}
	}
}

func TestChannelHealthOverall(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := channelHealthOverall(nil); got != "success" {
			t.Errorf("channelHealthOverall(nil) = %q, want success", got)
		}
	})
	t.Run("all_success", func(t *testing.T) {
		sites := []ChannelHealthSite{{Level: "success"}, {Level: "success"}}
		if got := channelHealthOverall(sites); got != "success" {
			t.Errorf("got %q, want success", got)
		}
	})
	t.Run("warning_present", func(t *testing.T) {
		sites := []ChannelHealthSite{{Level: "success"}, {Level: "warning"}}
		if got := channelHealthOverall(sites); got != "warning" {
			t.Errorf("got %q, want warning", got)
		}
	})
	t.Run("danger_short_circuits", func(t *testing.T) {
		sites := []ChannelHealthSite{{Level: "warning"}, {Level: "danger"}, {Level: "success"}}
		if got := channelHealthOverall(sites); got != "danger" {
			t.Errorf("got %q, want danger (short-circuit)", got)
		}
	})
}

func TestChannelHealthSiteAdvice(t *testing.T) {
	cases := []struct {
		name         string
		site         ChannelHealthSite
		wantLevel    string
		wantContains string
	}{
		{
			name:         "unreachable",
			site:         ChannelHealthSite{HealthStatus: "unreachable"},
			wantLevel:    "danger",
			wantContains: "暂停",
		},
		{
			name:         "down_status",
			site:         ChannelHealthSite{HealthStatus: "down"},
			wantLevel:    "danger",
			wantContains: "暂停",
		},
		{
			name:         "invalid_key",
			site:         ChannelHealthSite{HealthStatus: "healthy", InvalidKeyCount: 2},
			wantLevel:    "danger",
			wantContains: "无效 Key",
		},
		{
			name:         "failed_models",
			site:         ChannelHealthSite{HealthStatus: "healthy", FailedModelChannelCount: 1},
			wantLevel:    "warning",
			wantContains: "重新同步",
		},
		{
			name:         "unchecked_keys",
			site:         ChannelHealthSite{HealthStatus: "healthy", UncheckedKeyCount: 3},
			wantLevel:    "warning",
			wantContains: "补跑",
		},
		{
			name:         "unknown_status",
			site:         ChannelHealthSite{HealthStatus: "unknown"},
			wantLevel:    "warning",
			wantContains: "补齐",
		},
		{
			name:         "all_healthy",
			site:         ChannelHealthSite{HealthStatus: "healthy"},
			wantLevel:    "success",
			wantContains: "保持",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			level, action := channelHealthSiteAdvice(tc.site)
			if level != tc.wantLevel {
				t.Errorf("level = %q, want %q", level, tc.wantLevel)
			}
			if !contains(action, tc.wantContains) {
				t.Errorf("action = %q, want substring %q", action, tc.wantContains)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestBuildChannelHealthOverview(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ov := buildChannelHealthOverview(nil, nil, "2026-07-01T00:00:00Z")
		if ov.SiteCount != 0 || ov.ChannelCount != 0 || ov.Overall != "success" {
			t.Errorf("unexpected overview: %+v", ov)
		}
		if len(ov.Sites) != 0 {
			t.Errorf("expected 0 sites, got %d", len(ov.Sites))
		}
	})

	t.Run("site_status_counts", func(t *testing.T) {
		sites := []ChannelHealthSiteRow{
			{SiteID: "s1", SiteName: "Alpha", BaseURL: "https://a.com", HealthStatus: "healthy", ValidKeyCount: 2, InvalidKeyCount: 1},
			{SiteID: "s2", SiteName: "Beta", BaseURL: "https://b.com", HealthStatus: "unreachable"},
		}
		ov := buildChannelHealthOverview(sites, nil, "now")
		if ov.SiteCount != 2 {
			t.Errorf("SiteCount = %d, want 2", ov.SiteCount)
		}
		if ov.HealthySiteCount != 1 {
			t.Errorf("HealthySiteCount = %d, want 1", ov.HealthySiteCount)
		}
		if ov.UnreachableSiteCount != 1 {
			t.Errorf("UnreachableSiteCount = %d, want 1", ov.UnreachableSiteCount)
		}
		if ov.ValidKeyCount != 2 || ov.InvalidKeyCount != 1 {
			t.Errorf("key counts: valid=%d invalid=%d", ov.ValidKeyCount, ov.InvalidKeyCount)
		}
		if ov.Overall != "danger" {
			t.Errorf("Overall = %q, want danger", ov.Overall)
		}
	})

	t.Run("model_rows_join_by_site_id", func(t *testing.T) {
		sites := []ChannelHealthSiteRow{
			{SiteID: "s1", SiteName: "Alpha", BaseURL: "https://a.com", HealthStatus: "healthy"},
		}
		models := []ChannelHealthModelRow{
			{SiteID: "s1", Status: "live_key", ModelCount: 5, ChannelName: "ch1"},
			{SiteID: "s1", Status: "failed", ModelCount: 0, ChannelName: "ch2", Message: "key invalid"},
			{SiteID: "s1", Status: "unchecked", ModelCount: 0, ChannelName: "ch3"},
		}
		ov := buildChannelHealthOverview(sites, models, "now")
		if ov.ChannelCount != 3 {
			t.Errorf("ChannelCount = %d, want 3", ov.ChannelCount)
		}
		if ov.LiveModelChannelCount != 1 {
			t.Errorf("LiveModelChannelCount = %d, want 1", ov.LiveModelChannelCount)
		}
		if ov.FailedModelChannelCount != 1 {
			t.Errorf("FailedModelChannelCount = %d, want 1", ov.FailedModelChannelCount)
		}
		if ov.UncheckedModelChannelCount != 1 {
			t.Errorf("UncheckedModelChannelCount = %d, want 1", ov.UncheckedModelChannelCount)
		}
		if len(ov.Sites) != 1 || ov.Sites[0].ModelChannelCount != 3 {
			t.Errorf("site model channel count wrong: %+v", ov.Sites)
		}
	})

	t.Run("model_rows_join_by_base_url_when_site_id_missing", func(t *testing.T) {
		sites := []ChannelHealthSiteRow{
			{SiteID: "s1", SiteName: "Alpha", BaseURL: "https://a.com/", HealthStatus: "healthy"},
		}
		models := []ChannelHealthModelRow{
			{SiteID: "", BaseURL: "https://a.com", Status: "live_key", ChannelName: "ch1"},
		}
		ov := buildChannelHealthOverview(sites, models, "now")
		if ov.LiveModelChannelCount != 1 {
			t.Errorf("expected model row to join via base_url, LiveModelChannelCount=%d", ov.LiveModelChannelCount)
		}
	})

	t.Run("model_rows_with_unknown_site_skipped", func(t *testing.T) {
		sites := []ChannelHealthSiteRow{
			{SiteID: "s1", SiteName: "Alpha", BaseURL: "https://a.com", HealthStatus: "healthy"},
		}
		models := []ChannelHealthModelRow{
			{SiteID: "unknown", BaseURL: "https://other.com", Status: "live_key", ChannelName: "ch1"},
		}
		ov := buildChannelHealthOverview(sites, models, "now")
		// ChannelCount increments before the site lookup
		if ov.ChannelCount != 1 {
			t.Errorf("ChannelCount = %d, want 1", ov.ChannelCount)
		}
		if ov.LiveModelChannelCount != 0 {
			t.Errorf("LiveModelChannelCount = %d, want 0 (site not found)", ov.LiveModelChannelCount)
		}
	})

	t.Run("sites_truncated_at_80", func(t *testing.T) {
		sites := make([]ChannelHealthSiteRow, 90)
		for i := range sites {
			sites[i] = ChannelHealthSiteRow{SiteID: "s" + itoa(i), SiteName: "Site" + itoa(i), HealthStatus: "healthy"}
		}
		ov := buildChannelHealthOverview(sites, nil, "now")
		if len(ov.Sites) != 80 {
			t.Errorf("len(Sites) = %d, want 80", len(ov.Sites))
		}
	})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
