package versioncheck

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.0", "v1.0", 0},
		{"1.0.0", "1.0.0", 0},
		{"v1.0", "v2.0", -1},
		{"v2.0", "v1.0", 1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1", "v1.0.9", 1},
		{"v1.0.9", "v1.1", -1},
		{"2.0.0", "2.0.1", -1},
		{"v1.0", "1.0", 0},
		{"", "", 0},
		{"v1.0.0", "v1.0.0.1", -1},
		{"v1.0.0.1", "v1.0.0", 1},
	}
	for _, tt := range tests {
		got := CompareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
