package core

import "testing"

func TestIsExcludedRelaySite(t *testing.T) {
	tests := []struct {
		name     string
		siteName string
		baseURL  string
		want     bool
	}{
		{"exact token in name", "9router VIP", "https://example.com", true},
		{"exact token in baseURL", "My Site", "https://freemodel.example.com", true},
		{"case insensitive name", "FreeModel Hub", "https://example.com", true},
		{"case insensitive baseURL", "My Site", "https://TokenRouter.io", true},
		{"space separated token in name", "free model depot", "https://example.com", true},
		{"space separated token in baseURL", "My Site", "https://token router.example.com", true},
		{"no match", "Alpha Relay", "https://alpha.example.com", false},
		{"empty name and baseURL", "", "", false},
		{"partial word no match", "9route", "https://example.com", false},
		{"whitespace only inputs", "   ", "   ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isExcludedRelaySite(tc.siteName, tc.baseURL)
			if got != tc.want {
				t.Errorf("isExcludedRelaySite(%q, %q) = %v, want %v", tc.siteName, tc.baseURL, got, tc.want)
			}
		})
	}
}

func TestExcludedRelaySiteMatch(t *testing.T) {
	tests := []struct {
		name       string
		siteName   string
		baseURL    string
		wantToken  string
		wantMatch  bool
	}{
		{"9router in name", "9router VIP", "https://example.com", "9router", true},
		{"freemodel in baseURL", "My Site", "https://freemodel.io", "freemodel", true},
		{"free model with space in name", "free model depot", "https://x.com", "free model", true},
		{"token router with space in baseURL", "Site", "https://token router.dev", "token router", true},
		{"tokenrouter in name", "TokenRouter Hub", "https://x.com", "tokenrouter", true},
		{"case insensitive match preserves original token", "FreeModel Hub", "https://x.com", "freemodel", true},
		{"first token wins when multiple match", "9router freemodel combo", "https://x.com", "9router", true},
		{"no match returns empty", "Clean Site", "https://clean.example.com", "", false},
		{"empty inputs", "", "", "", false},
		{"whitespace padded name matched", "  9router  ", "https://x.com", "9router", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotToken, gotMatch := excludedRelaySiteMatch(tc.siteName, tc.baseURL)
			if gotToken != tc.wantToken || gotMatch != tc.wantMatch {
				t.Errorf("excludedRelaySiteMatch(%q, %q) = (%q, %v), want (%q, %v)",
					tc.siteName, tc.baseURL, gotToken, gotMatch, tc.wantToken, tc.wantMatch)
			}
		})
	}
}

func TestIsManagedRelayKind(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want bool
	}{
		{"newapi lowercase", "newapi", true},
		{"oneapi lowercase", "oneapi", true},
		{"sub2api lowercase", "sub2api", true},
		{"modified_relay lowercase", "modified_relay", true},
		{"NewAPI mixed case", "NewAPI", true},
		{"OneAPI mixed case", "OneAPI", true},
		{"Sub2API mixed case", "Sub2API", true},
		{"Modified_Relay mixed case", "Modified_Relay", true},
		{"NEWAPI all caps", "NEWAPI", true},
		{"unknown kind", "relay", false},
		{"empty kind", "", false},
		{"whitespace kind", "  ", false},
		{"kind with leading space", " newapi", true},
		{"kind with trailing space", "oneapi ", true},
		{"partial match newapi_", "newapi_v2", false},
		{"foreign relay kind", "aiproxy", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isManagedRelayKind(tc.kind)
			if got != tc.want {
				t.Errorf("isManagedRelayKind(%q) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}
