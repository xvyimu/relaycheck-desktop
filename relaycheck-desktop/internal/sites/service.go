package sites

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// globalScheduleSiteID mirrors the host's virtual site ID used for the global
// checkin schedule. It must stay in sync with core.channel_schedules.
const globalScheduleSiteID = "__global__"

// Infra is the subset of the host application that the sites domain depends
// on. Extracting it breaks the reverse reference from the sites service back
// to the host god object. The host (e.g. *core.App) satisfies this interface
// by providing database access, HTTP client, outbound URL validation, and
// lifecycle hooks (notify/audit/id/time).
//
// All methods are exported so that types defined in other packages (the host
// application) can satisfy the interface cross-package.
type Infra interface {
	// DB returns the application's SQLite database handle.
	DB() *sql.DB
	// DoHTTP executes req with the host's default HTTP client (honouring
	// proxy config and test-mode client overrides).
	DoHTTP(req *http.Request) (*http.Response, error)
	// ValidateOutboundURL validates raw against the host's external outbound
	// policy (SSRF defences, controlled by allowLocalOutbound). Used by
	// DetectUpstream.
	ValidateOutboundURL(ctx context.Context, raw string) (*url.URL, error)
	// ValidateLocalURL validates raw against a permissive policy that allows
	// loopback addresses. Used by ProbeLocal when scanning local NewAPI
	// instances.
	ValidateLocalURL(ctx context.Context, raw string) (*url.URL, error)
	// AllowLocalOutbound reports whether the host permits loopback outbound
	// connections (controls DetectUpstream's URL policy).
	AllowLocalOutbound() bool
	// Notify dispatches a notification through the host's notification hub.
	Notify(kind, level, title, content, relatedType, relatedID string)
	// Audit records an audit log entry.
	Audit(action, level, userID, entityType, entityID, detail string, metadata map[string]interface{})
	// Now returns the host's current timestamp string (ISO 8601).
	Now() string
	// NewID returns a fresh host-generated identifier.
	NewID() string
}

// Service implements the site CRUD + upstream detection + local scan domain.
// It owns the upstream_sites table round-tripping and the probe-based
// detection engine, while relying on Infra for database access, HTTP, URL
// validation, and lifecycle hooks. The host application delegates its *App
// handler/forwarding methods to this Service.
type Service struct {
	infra Infra
}

// NewService constructs a sites Service backed by the given Infra.
func NewService(infra Infra) *Service {
	return &Service{infra: infra}
}

// ListUpstreamSites returns all upstream sites ordered by updated_at desc,
// excluding the virtual global-schedule record. Each site's account count is
// populated via a correlated subquery. Official-provider sites are normalized
// to kind=official_provider.
func (s *Service) ListUpstreamSites(ctx context.Context) ([]Site, error) {
	rows, err := s.infra.DB().QueryContext(ctx, `
		SELECT s.id, COALESCE(s.channel_id,''), s.name, COALESCE(s.homepage_url,''), s.base_url,
		       COALESCE(s.login_url,''), s.kind, s.detection_confidence, s.health_status,
		       s.supports_checkin, s.supports_balance, s.supports_models, s.supports_pricing,
		       COALESCE(s.detection_json,''), COALESCE(s.last_health_check_at,''), s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM channel_accounts a WHERE a.upstream_site_id = s.id)
		FROM upstream_sites s
		WHERE s.id <> ?
		ORDER BY s.updated_at DESC
	`, globalScheduleSiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Site{}
	for rows.Next() {
		var item Site
		var checkin, balance, models, pricing int
		if err := rows.Scan(&item.ID, &item.ChannelID, &item.Name, &item.HomepageURL, &item.BaseURL, &item.LoginURL, &item.Kind, &item.DetectionConfidence, &item.HealthStatus, &checkin, &balance, &models, &pricing, &item.DetectionJSON, &item.LastHealthCheckAt, &item.CreatedAt, &item.UpdatedAt, &item.AccountCount); err != nil {
			return nil, err
		}
		item.SupportsCheckin = checkin == 1
		item.SupportsBalance = balance == 1
		item.SupportsModels = models == 1
		item.SupportsPricing = pricing == 1
		normalizeOfficialProviderSite(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateUpstreamSite inserts a manually-created upstream site (plus its
// shadow imported_channels row), emits a notification, and returns the new
// site ID. The caller is responsible for running detection and kind
// validation before calling this method.
func (s *Service) CreateUpstreamSite(ctx context.Context, input CreateSiteInput, detection Detection) (string, error) {
	channelID := s.infra.NewID()
	siteID := s.infra.NewID()
	detectionJSON := MarshalDetection(&detection)
	_, err := s.infra.DB().ExecContext(ctx, `
		INSERT INTO imported_channels (id, source_channel_id, name, base_url, status, upstream_kind, supports_checkin, supports_balance, supports_models, supports_pricing, raw_json, detection_json, last_detected_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, channelID, "manual-"+channelID, input.Name, detection.BaseURL, detection.Kind, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), `{"source":"manual"}`, detectionJSON, s.infra.Now(), s.infra.Now(), s.infra.Now())
	if err != nil {
		return "", err
	}

	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, detection_confidence, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_json, last_health_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, siteID, channelID, input.Name, detection.HomepageURL, detection.BaseURL, detection.LoginURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), detectionJSON, s.infra.Now(), s.infra.Now(), s.infra.Now())
	if err != nil {
		return "", err
	}
	s.infra.Notify("upstream_site_created", "success", "上游站点已添加", input.Name+" 已加入站点列表。", "upstream_site", siteID)
	return siteID, nil
}

// DetectUpstreamSite re-probes the base URL of an existing site, persists the
// refreshed detection, and returns the new Detection.
func (s *Service) DetectUpstreamSite(ctx context.Context, id string) (Detection, error) {
	var baseURL string
	err := s.infra.DB().QueryRowContext(ctx, `SELECT base_url FROM upstream_sites WHERE id = ?`, id).Scan(&baseURL)
	if err == sql.ErrNoRows {
		return Detection{}, sql.ErrNoRows
	}
	if err != nil {
		return Detection{}, err
	}
	detection := s.DetectUpstream(ctx, baseURL)
	_, err = s.infra.DB().ExecContext(ctx, `
		UPDATE upstream_sites
		SET homepage_url=?, base_url=?, kind=?, detection_confidence=?, health_status=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_health_check_at=?, updated_at=?
		WHERE id=?
	`, detection.HomepageURL, detection.BaseURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), MarshalDetection(&detection), s.infra.Now(), s.infra.Now(), id)
	if err != nil {
		return Detection{}, err
	}
	return detection, nil
}

// DeleteUpstreamSite removes a site row and records an audit entry.
func (s *Service) DeleteUpstreamSite(ctx context.Context, id string) error {
	_, err := s.infra.DB().ExecContext(ctx, `DELETE FROM upstream_sites WHERE id = ?`, id)
	if err != nil {
		return err
	}
	s.infra.Audit("upstream_site.deleted", "warning", "", "upstream_site", id, "上游站点已删除", nil)
	return nil
}

// DetectAndSaveSite re-probes a site, persists the detection, and returns the
// per-site result used by the bulk-detect endpoint.
func (s *Service) DetectAndSaveSite(ctx context.Context, id string, name string, baseURL string) BulkDetectResult {
	result := BulkDetectResult{ID: id, Name: name, BaseURL: baseURL}
	detection := s.DetectUpstream(ctx, baseURL)
	_, err := s.infra.DB().ExecContext(ctx, `
		UPDATE upstream_sites
		SET homepage_url=?, base_url=?, kind=?, detection_confidence=?, health_status=?, supports_checkin=?, supports_balance=?, supports_models=?, supports_pricing=?, detection_json=?, last_health_check_at=?, updated_at=?
		WHERE id=?
	`, detection.HomepageURL, detection.BaseURL, detection.Kind, detection.DetectionConfidence, detection.HealthStatus, boolInt(detection.SupportsCheckin), boolInt(detection.SupportsBalance), boolInt(detection.SupportsModels), boolInt(detection.SupportsPricing), MarshalDetection(&detection), s.infra.Now(), s.infra.Now(), id)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.BaseURL = detection.BaseURL
	result.Kind = detection.Kind
	result.HealthStatus = detection.HealthStatus
	result.SupportsCheckin = detection.SupportsCheckin
	return result
}

// EnsureUpstreamSiteForChannel upserts an upstream_sites row for the given
// channel. If a site with the same base URL exists, it is updated; otherwise
// a new row is inserted. Returns the site ID and whether a new row was
// created. Sites whose detection kind is not a managed relay kind are
// skipped (returning "", false, nil) to mirror the original behaviour.
func (s *Service) EnsureUpstreamSiteForChannel(ctx context.Context, input EnsureSiteInput) (string, bool, error) {
	baseURL := normalizeBaseURL(input.RawBaseURL)
	if baseURL == "" {
		return "", false, nil
	}

	homepageURL := baseURL
	loginURL := strings.TrimRight(baseURL, "/") + "/login"
	healthStatus := "unknown"
	confidence := 0.0
	supportsCheckin := false
	supportsBalance := false
	supportsModels := false
	supportsPricing := false
	kind := input.Kind
	if kind == "" {
		kind = "unknown"
	}
	if input.Detection != nil {
		homepageURL = input.Detection.HomepageURL
		loginURL = input.Detection.LoginURL
		kind = input.Detection.Kind
		healthStatus = input.Detection.HealthStatus
		confidence = input.Detection.DetectionConfidence
		supportsCheckin = input.Detection.SupportsCheckin
		supportsBalance = input.Detection.SupportsBalance
		supportsModels = input.Detection.SupportsModels
		supportsPricing = input.Detection.SupportsPricing
	}
	detectionJSON := MarshalDetection(input.Detection)
	if input.Detection != nil && !IsManagedRelayKind(kind) {
		return "", false, nil
	}

	var siteID string
	err := s.infra.DB().QueryRowContext(ctx, `SELECT id FROM upstream_sites WHERE base_url=? ORDER BY updated_at DESC LIMIT 1`, baseURL).Scan(&siteID)
	if err == nil {
		_, err = s.infra.DB().ExecContext(ctx, `
			UPDATE upstream_sites
			SET channel_id=CASE WHEN COALESCE(channel_id,'')='' THEN ? ELSE channel_id END,
			    name=CASE WHEN name='' THEN ? ELSE name END,
			    homepage_url=?,
			    login_url=CASE WHEN COALESCE(login_url,'')='' THEN ? ELSE login_url END,
			    kind=CASE WHEN kind='unknown' THEN ? ELSE kind END,
			    detection_confidence=MAX(detection_confidence, ?),
			    detection_json=CASE WHEN ?='' THEN detection_json ELSE ? END,
			    health_status=CASE WHEN ?='unknown' THEN health_status ELSE ? END,
			    supports_checkin=MAX(supports_checkin, ?),
			    supports_balance=MAX(supports_balance, ?),
			    supports_models=MAX(supports_models, ?),
			    supports_pricing=MAX(supports_pricing, ?),
			    last_health_check_at=?,
			    updated_at=?
			WHERE id=?
		`, input.ChannelID, input.Name, homepageURL, loginURL, kind, confidence, detectionJSON, detectionJSON, healthStatus, healthStatus, boolInt(supportsCheckin), boolInt(supportsBalance), boolInt(supportsModels), boolInt(supportsPricing), s.infra.Now(), s.infra.Now(), siteID)
		return siteID, false, err
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	siteID = s.infra.NewID()
	_, err = s.infra.DB().ExecContext(ctx, `
		INSERT INTO upstream_sites (id, channel_id, name, homepage_url, base_url, login_url, kind, detection_confidence, health_status, supports_checkin, supports_balance, supports_models, supports_pricing, detection_json, last_health_check_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, siteID, input.ChannelID, input.Name, homepageURL, baseURL, loginURL, kind, confidence, healthStatus, boolInt(supportsCheckin), boolInt(supportsBalance), boolInt(supportsModels), boolInt(supportsPricing), detectionJSON, s.infra.Now(), s.infra.Now(), s.infra.Now())
	return siteID, true, err
}

// MarshalDetection serializes a Detection to its JSON form. Returns "" for a
// nil detection so callers can store the empty string directly.
func MarshalDetection(detection *Detection) string {
	if detection == nil {
		return ""
	}
	payload, err := json.Marshal(detection)
	if err != nil {
		return ""
	}
	return string(payload)
}

// NormalizeOfficialProviderSite forces official-provider sites to a canonical
// kind/health. Exported so the host can reuse it for its loadSiteDetail
// aggregation without re-implementing.
func NormalizeOfficialProviderSite(item *Site) {
	normalizeOfficialProviderSite(item)
}

func normalizeOfficialProviderSite(item *Site) {
	if !isOfficialProviderBaseURL(item.BaseURL) {
		return
	}
	item.Kind = "official_provider"
	item.SupportsCheckin = false
	if item.HealthStatus == "" || item.HealthStatus == "unknown" {
		item.HealthStatus = "healthy"
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
