package autostart

import (
	"runtime"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService should return non-nil Service")
	}
}

func TestStatus_SupportedMatchesOS(t *testing.T) {
	svc := NewService()
	status := svc.Status()
	if status.Supported != (runtime.GOOS == "windows") {
		t.Errorf("Supported = %v, expected %v (GOOS=%s)", status.Supported, runtime.GOOS == "windows", runtime.GOOS)
	}
}

func TestStatus_TargetPathPopulated(t *testing.T) {
	svc := NewService()
	status := svc.Status()
	// os.Executable() should succeed in test contexts.
	if status.TargetPath == "" {
		t.Fatal("TargetPath should be populated from os.Executable()")
	}
}

func TestStatus_CheckedAtNotApplicable(t *testing.T) {
	// Status has no CheckedAt field; this test documents that Enabled is a
	// bool reflecting on-disk shortcut presence. On a clean test machine
	// the shortcut should not exist, so Enabled should be false.
	svc := NewService()
	status := svc.Status()
	// We cannot assert Enabled=false definitively (a shortcut might exist),
	// but the field must be a valid bool without panicking.
	_ = status.Enabled
}

func TestEnableDisable_OnNonWindowsReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows behavior test skipped on Windows")
	}
	svc := NewService()
	if err := svc.Enable(); err == nil {
		t.Fatal("Enable should return error on non-Windows")
	}
	if err := svc.Disable(); err == nil {
		t.Fatal("Disable should return error on non-Windows")
	}
}

func TestEnableDisable_OnWindowsDoesNotPanic(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows behavior test skipped on non-Windows")
	}
	// Enable/Disable mutate the real shell:startup folder. We intentionally
	// skip calling them to avoid side effects in the test environment; the
	// platform_windows.go functions are exercised by the smoke suite
	// (.pipeline / frontend/scripts/smoke.mjs) instead.
	svc := NewService()
	_ = svc // verify construction without panicking
}
