//go:build windows

package autostart

import (
	"path/filepath"
	"testing"
)

func TestHiddenProcessAttrHidesWindow(t *testing.T) {
	attr := hiddenProcessAttr()
	if attr == nil {
		t.Fatal("hiddenProcessAttr returned nil")
	}
	if !attr.HideWindow {
		t.Fatal("hiddenProcessAttr should set HideWindow")
	}
}

func TestEscapePowerShellSingleQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", `C:\RelayCheck\relaycheck.exe`, `C:\RelayCheck\relaycheck.exe`},
		{"single-quote", `C:\Users\O'Brien\relaycheck.exe`, `C:\Users\O''Brien\relaycheck.exe`},
		{"multiple-quotes", `it's 'quoted'`, `it''s ''quoted''`},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapePowerShellSingleQuote(tc.in); got != tc.want {
				t.Fatalf("escapePowerShellSingleQuote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStartupFolderRequiresAppData(t *testing.T) {
	t.Setenv("APPDATA", "")

	if got, err := startupFolder(); err == nil {
		t.Fatalf("startupFolder() = %q, want APPDATA error", got)
	}
}

func TestShortcutPathUsesStartupFolder(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "Roaming")
	t.Setenv("APPDATA", appData)

	got, err := shortcutPath()
	if err != nil {
		t.Fatalf("shortcutPath() error = %v", err)
	}

	want := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", startupShortcutFileName)
	if got != want {
		t.Fatalf("shortcutPath() = %q, want %q", got, want)
	}
}
