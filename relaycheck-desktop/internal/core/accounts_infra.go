package core

import (
	"context"

	"relaycheck-desktop/internal/accounts"
)

// This file hosts the exported adapter methods that *App implements to
// satisfy accounts.Infra. The accounts service calls these methods; they
// forward to the host's internal helpers and convert the host's private
// types (UpstreamDetection) into the accounts package's mirror types.
//
// Method names that would collide with adapters already provided for other
// packages (channels.DetectUpstream, channels.EnsureUpstreamSiteForChannel)
// use distinct "ForImport" suffixes so a single *App can satisfy multiple
// packages' Infra interfaces without Go method-name conflicts.
//
// Methods already provided by *App for other packages (DB, DoHTTP, Notify,
// Audit, Now, NewID, DecryptText) are reused unchanged — see app.go,
// infra.go, channels_infra.go, and network.go.

// EncryptText is the exported adapter for the accounts package's Infra
// interface. It delegates to the host's crypto service so the accounts
// service can persist channel keys / account passwords without importing
// core.
func (a *App) EncryptText(plaintext string) (string, error) {
	return a.crypto.Encrypt(plaintext)
}

// DetectUpstreamForImport is the exported adapter for the accounts package's
// Infra interface. It forwards to the host's sites-backed detectUpstream
// helper and converts the core.UpstreamDetection to accounts.Detection so
// the accounts service stays decoupled from core. Distinct from
// channels.DetectUpstream (channels_infra.go) by the "ForImport" suffix.
func (a *App) DetectUpstreamForImport(ctx context.Context, raw string) (accounts.Detection, error) {
	d := a.detectUpstream(ctx, raw)
	return accounts.Detection{
		BaseURL:             d.BaseURL,
		HomepageURL:         d.HomepageURL,
		LoginURL:            d.LoginURL,
		Kind:                d.Kind,
		HealthStatus:        d.HealthStatus,
		DetectionConfidence: d.DetectionConfidence,
		SupportsCheckin:     d.SupportsCheckin,
		SupportsBalance:     d.SupportsBalance,
		SupportsModels:      d.SupportsModels,
		SupportsPricing:     d.SupportsPricing,
		MatchedSignals:      d.MatchedSignals,
	}, nil
}

// EnsureChannelSiteForImport is the exported adapter for the accounts
// package's Infra interface. It forwards to the host's sites-backed
// ensureUpstreamSiteForChannel helper, converting the accounts.Detection
// into the (*UpstreamDetection) the host helper expects. Distinct from
// channels.EnsureUpstreamSiteForChannel (channels_infra.go) by the
// "ForImport" suffix.
func (a *App) EnsureChannelSiteForImport(ctx context.Context, channelID, name, rawBaseURL, kind string, detection *accounts.Detection) (string, bool, error) {
	var coreDetection *UpstreamDetection
	if detection != nil {
		coreDetection = &UpstreamDetection{
			BaseURL:             detection.BaseURL,
			HomepageURL:         detection.HomepageURL,
			LoginURL:            detection.LoginURL,
			Kind:                detection.Kind,
			HealthStatus:        detection.HealthStatus,
			DetectionConfidence: detection.DetectionConfidence,
			SupportsCheckin:     detection.SupportsCheckin,
			SupportsBalance:     detection.SupportsBalance,
			SupportsModels:      detection.SupportsModels,
			SupportsPricing:     detection.SupportsPricing,
			MatchedSignals:      detection.MatchedSignals,
		}
	}
	return a.ensureUpstreamSiteForChannel(ctx, channelID, name, rawBaseURL, kind, coreDetection)
}
