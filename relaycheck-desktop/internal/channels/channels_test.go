package channels

import "testing"

func TestIsSafeBulkSourceStatusTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"missing", "archived", true},
		{"missing", "active", true},
		{"archived", "active", true},
		{"active", "archived", false},
		{"archived", "missing", false},
		{"active", "missing", false},
		{"", "", false},
		{"missing", "", false},
		{"", "active", false},
	}
	for _, tc := range cases {
		key := tc.from + "->" + tc.to
		t.Run(key, func(t *testing.T) {
			if got := IsSafeBulkSourceStatusTransition(tc.from, tc.to); got != tc.want {
				t.Errorf("IsSafeBulkSourceStatusTransition(%q, %q) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}
