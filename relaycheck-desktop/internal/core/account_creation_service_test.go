package core

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAccountCreationServiceCreateStoresEncryptedCredentialsAndDefaults(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Create Site', 'https://create.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	id, err := NewAccountCreationService(app).Create(context.Background(), accountCreationInput{
		UpstreamSiteID: siteID,
		Email:          " user@example.com ",
		Username:       "username",
		Password:       "secret-password",
		APIKey:         "sk-create-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("Create returned empty id")
	}

	var displayName, authType, loginStatus, apiKeyStatus, apiKeyFingerprint, passwordEncrypted, apiKeyEncrypted string
	if err := app.db.QueryRow(`
		SELECT display_name, auth_type, login_status, api_key_status, api_key_fingerprint, password_encrypted, api_key_encrypted
		FROM channel_accounts WHERE id=?
	`, id).Scan(&displayName, &authType, &loginStatus, &apiKeyStatus, &apiKeyFingerprint, &passwordEncrypted, &apiKeyEncrypted); err != nil {
		t.Fatal(err)
	}
	if displayName != "user@example.com" {
		t.Fatalf("displayName = %q, want trimmed email default", displayName)
	}
	if authType != "api_key" || loginStatus != "unknown" {
		t.Fatalf("auth/login = %q/%q, want api_key/unknown", authType, loginStatus)
	}
	if apiKeyFingerprint != secretFingerprint("sk-create-test") || apiKeyStatus != "unchecked" {
		t.Fatalf("api key metadata = %q/%q, want fingerprint/unchecked", apiKeyFingerprint, apiKeyStatus)
	}
	password, err := app.decryptText(passwordEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	apiKey, err := app.decryptText(apiKeyEncrypted)
	if err != nil {
		t.Fatal(err)
	}
	if password != "secret-password" || apiKey != "sk-create-test" {
		t.Fatalf("decrypted credentials = %q/%q, want original values", password, apiKey)
	}
}

func TestAccountCreationServiceCreateBrowserProfileDefaultsManualRequired(t *testing.T) {
	app := newTestApp(t)

	siteID := newID()
	if _, err := app.db.Exec(`
		INSERT INTO upstream_sites (id, name, base_url, kind, health_status, created_at, updated_at)
		VALUES (?, 'Browser Site', 'https://browser.example', 'newapi', 'healthy', ?, ?)
	`, siteID, now(), now()); err != nil {
		t.Fatal(err)
	}

	id, err := NewAccountCreationService(app).Create(context.Background(), accountCreationInput{UpstreamSiteID: siteID})
	if err != nil {
		t.Fatal(err)
	}

	var authType, loginStatus, profilePath string
	if err := app.db.QueryRow(`
		SELECT auth_type, login_status, browser_profile_path
		FROM channel_accounts WHERE id=?
	`, id).Scan(&authType, &loginStatus, &profilePath); err != nil {
		t.Fatal(err)
	}
	if authType != "browser_profile" || loginStatus != "manual_required" {
		t.Fatalf("auth/login = %q/%q, want browser_profile/manual_required", authType, loginStatus)
	}
	wantProfile := filepath.Join(app.dataDir, "browser-profiles", id)
	if profilePath != wantProfile {
		t.Fatalf("profilePath = %q, want %q", profilePath, wantProfile)
	}
}

func TestAccountCreationServiceEnsureManualSiteUsesInjectedDetectionAndManualLogin(t *testing.T) {
	app := newTestApp(t)

	service := NewAccountCreationService(app)
	var detectedBaseURL string
	service.detectUpstream = func(ctx context.Context, baseURL string) UpstreamDetection {
		detectedBaseURL = baseURL
		return UpstreamDetection{
			BaseURL:             baseURL,
			HomepageURL:         baseURL + "/",
			LoginURL:            baseURL + "/auto-login",
			LoginDiscovery:      &LoginDiscovery{URL: baseURL + "/auto-login", Source: "html", Confidence: 0.8, Candidates: []string{baseURL + "/auto-login"}},
			Kind:                "newapi",
			HealthStatus:        "healthy",
			DetectionConfidence: 0.9,
			SupportsCheckin:     true,
			SupportsBalance:     true,
			SupportsModels:      true,
			SupportsPricing:     true,
		}
	}

	manualURL := "https://relay.example/manual-login"
	siteID, err := service.EnsureManualSite(context.Background(), "Relay", "https://relay.example/", manualURL, "newapi")
	if err != nil {
		t.Fatal(err)
	}
	if detectedBaseURL != "https://relay.example" {
		t.Fatalf("detected base URL = %q, want normalized base URL", detectedBaseURL)
	}

	var loginURL, source, discoveryJSON, detectionJSON, kind string
	var confidence float64
	if err := app.db.QueryRow(`
		SELECT login_url, login_url_source, login_url_confidence, COALESCE(login_discovery_json,''), COALESCE(detection_json,''), kind
		FROM upstream_sites WHERE id=?
	`, siteID).Scan(&loginURL, &source, &confidence, &discoveryJSON, &detectionJSON, &kind); err != nil {
		t.Fatal(err)
	}
	if loginURL != manualURL || source != "manual" || confidence != 1 || kind != "newapi" {
		t.Fatalf("site metadata = %q/%q/%.1f/%q, want manual newapi metadata", loginURL, source, confidence, kind)
	}
	manualDiscovery := parseLoginDiscoveryJSON(discoveryJSON)
	if manualDiscovery == nil || manualDiscovery.URL != manualURL || manualDiscovery.Source != "manual" || len(manualDiscovery.Candidates) < 2 {
		t.Fatalf("manual discovery = %#v, want manual URL with auto candidate", manualDiscovery)
	}
	detection, ok := parseDetectionJSON(detectionJSON)
	if !ok || detection.LoginDiscovery == nil || detection.LoginDiscovery.URL != "https://relay.example/auto-login" {
		t.Fatalf("detection snapshot = %#v ok=%v, want auto login discovery", detection, ok)
	}
}
