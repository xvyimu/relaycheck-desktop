package core

import (
	"net/http/httptest"
	"testing"
)

func TestSystemStatusUsesDesktopProductIdentity(t *testing.T) {
	app, err := NewApp(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	status, err := app.systemStatus(httptest.NewRequest("GET", "/api/system/status", nil))
	if err != nil {
		t.Fatal(err)
	}

	if status.ProductName != "RelayCheck Desktop" {
		t.Fatalf("expected desktop product name, got %q", status.ProductName)
	}
	if status.ProductVersion == "" || status.BuildTime == "" {
		t.Fatalf("expected version and build time, got version=%q build=%q", status.ProductVersion, status.BuildTime)
	}
	if status.LastDiagnostics.Overall == "" || status.LastDiagnostics.ItemCount == 0 {
		t.Fatalf("expected diagnostics summary, got %#v", status.LastDiagnostics)
	}
}
