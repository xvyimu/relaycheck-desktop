package sites

// Detection is the sites-package-local mirror of the host's UpstreamDetection
// type. The host (core.UpstreamDetection) stays in core/models.go per the
// extraction contract; sites.Service produces Detection values and the host's
// forwarding helpers convert them back to core.UpstreamDetection so existing
// call sites are unchanged.
type Detection struct {
	BaseURL             string   `json:"baseUrl"`
	HomepageURL         string   `json:"homepageUrl"`
	LoginURL            string   `json:"loginUrl"`
	Kind                string   `json:"kind"`
	HealthStatus        string   `json:"healthStatus"`
	DetectionConfidence float64  `json:"detectionConfidence"`
	SupportsCheckin     bool     `json:"supportsCheckin"`
	SupportsBalance     bool     `json:"supportsBalance"`
	SupportsModels      bool     `json:"supportsModels"`
	SupportsPricing     bool     `json:"supportsPricing"`
	MatchedSignals      []string `json:"matchedSignals"`
}

// ProbeResult is the sites-package-local mirror of the host's ProbeResult
// type. It describes a single local NewAPI instance probe outcome.
type ProbeResult struct {
	BaseURL        string   `json:"baseUrl"`
	Status         string   `json:"status"`
	Reachable      bool     `json:"reachable"`
	MatchedSignals []string `json:"matchedSignals"`
	Score          int      `json:"score"`
}

// Site is the sites-package-local mirror of the host's UpstreamSite type.
// The host keeps core.UpstreamSite in core/models.go; sites.Service returns
// Site values and the host's handlers convert them so the JSON shape and
// existing call sites are unchanged.
type Site struct {
	ID                  string  `json:"id"`
	ChannelID           string  `json:"channelId,omitempty"`
	Name                string  `json:"name"`
	HomepageURL         string  `json:"homepageUrl,omitempty"`
	BaseURL             string  `json:"baseUrl"`
	LoginURL            string  `json:"loginUrl,omitempty"`
	Kind                string  `json:"kind"`
	DetectionConfidence float64 `json:"detectionConfidence"`
	HealthStatus        string  `json:"healthStatus"`
	SupportsCheckin     bool    `json:"supportsCheckin"`
	SupportsBalance     bool    `json:"supportsBalance"`
	SupportsModels      bool    `json:"supportsModels"`
	SupportsPricing     bool    `json:"supportsPricing"`
	AccountCount        int     `json:"accountCount,omitempty"`
	DetectionJSON       string  `json:"detectionJson,omitempty"`
	LastHealthCheckAt   string  `json:"lastHealthCheckAt,omitempty"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

// BulkDetectResult captures the outcome of detecting and persisting a single
// site during a bulk detect run. It mirrors the host's bulkDetectSiteResult.
type BulkDetectResult struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BaseURL         string `json:"baseUrl"`
	Kind            string `json:"kind"`
	HealthStatus    string `json:"healthStatus"`
	SupportsCheckin bool   `json:"supportsCheckin"`
	Error           string `json:"error,omitempty"`
}

// CreateSiteInput is the payload for creating an upstream site manually.
type CreateSiteInput struct {
	Name     string
	BaseURL  string
	LoginURL string
	Kind     string
}

// EnsureSiteInput is the payload for EnsureUpstreamSiteForChannel.
type EnsureSiteInput struct {
	ChannelID  string
	Name       string
	RawBaseURL string
	Kind       string
	Detection  *Detection
}
