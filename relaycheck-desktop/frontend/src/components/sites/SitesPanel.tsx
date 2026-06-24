import { useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatConfidence, formatTime } from "@/lib/format";
import type { UpstreamSite } from "@/types";

type SitesPanelProps = {
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
};

function isUnhealthy(status: string) {
  return ["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(
    status.toLowerCase(),
  );
}

function capabilityLabel(enabled?: boolean) {
  return enabled ? "supported" : "unknown / no";
}

export function SitesPanel({ sites, onRefresh }: SitesPanelProps) {
  const [busyId, setBusyId] = useState("");
  const [message, setMessage] = useState("");

  const summary = useMemo(
    () => ({
      total: sites.length,
      healthy: sites.filter((site) => ["healthy", "ok", "success"].includes(site.healthStatus.toLowerCase())).length,
      checkinReady: sites.filter((site) => site.supportsCheckin).length,
      linkedAccounts: sites.reduce((total, site) => total + (site.accountCount || 0), 0),
    }),
    [sites],
  );

  async function detect(site: UpstreamSite) {
    setBusyId(site.id);
    setMessage("");
    try {
      await api(`/api/upstream-sites/${site.id}/detect`, { method: "POST" });
      await onRefresh();
      setMessage(`${site.name} detection completed.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Detection failed.");
    } finally {
      setBusyId("");
    }
  }

  return (
    <section className="sites-panel">
      <div className="channel-summary site-summary compact-summary">
        <div>
          <span>Sites</span>
          <strong>{summary.total}</strong>
        </div>
        <div>
          <span>Healthy</span>
          <strong>{summary.healthy}</strong>
        </div>
        <div>
          <span>Check-in ready</span>
          <strong>{summary.checkinReady}</strong>
        </div>
        <div>
          <span>Linked accounts</span>
          <strong>{summary.linkedAccounts}</strong>
        </div>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="site-grid">
        {sites.map((site) => {
          const capabilities: Array<{ label: string; enabled?: boolean }> = [
            { label: "Check-in", enabled: site.supportsCheckin },
            { label: "Balance", enabled: site.supportsBalance },
            { label: "Models", enabled: site.supportsModels },
            { label: "Pricing", enabled: site.supportsPricing },
          ];

          return (
            <article
              className={`site-card ${isUnhealthy(site.healthStatus) ? "is-unhealthy" : ""}`}
              key={site.id}
            >
              <div className="site-card-head">
                <div>
                  <span>{site.kind || "unknown"}</span>
                  <strong title={site.name}>{site.name}</strong>
                </div>
                <span className={`status-pill ${isUnhealthy(site.healthStatus) ? "danger" : "neutral"}`}>
                  {site.healthStatus || "unknown"}
                </span>
              </div>

              <dl className="site-addresses">
                <div>
                  <dt>Base URL</dt>
                  <dd title={site.baseUrl}>{site.baseUrl || "-"}</dd>
                </div>
                <div>
                  <dt>Login URL</dt>
                  <dd title={site.loginUrl || ""}>{site.loginUrl || "-"}</dd>
                </div>
                {site.homepageUrl ? (
                  <div>
                    <dt>Home</dt>
                    <dd title={site.homepageUrl}>{site.homepageUrl}</dd>
                  </div>
                ) : null}
              </dl>

              <div className="site-card-metrics">
                <div>
                  <span>Accounts</span>
                  <strong>{site.accountCount || 0}</strong>
                </div>
                <div>
                  <span>Confidence</span>
                  <strong>{formatConfidence(site.detectionConfidence)}</strong>
                </div>
                <div>
                  <span>Last health</span>
                  <strong>{formatTime(site.lastHealthCheckAt || "")}</strong>
                </div>
              </div>

              <div className="chips secondary-chips">
                {capabilities.map(({ label, enabled }) => (
                  <span key={label}>
                    {label} {capabilityLabel(Boolean(enabled))}
                  </span>
                ))}
              </div>

              {site.updatedAt ? (
                <div className="channel-subtle">Updated {formatTime(site.updatedAt)}</div>
              ) : null}

              <div className="site-actions">
                <button
                  disabled={busyId === site.id}
                  onClick={() => void detect(site)}
                  type="button"
                >
                  {busyId === site.id ? "Detecting..." : "Detect capabilities"}
                </button>
              </div>
            </article>
          );
        })}

        {!sites.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>No upstream sites yet</strong>
            <span>Import NewAPI or OneAPI channels first, then detect site capabilities here.</span>
          </div>
        ) : null}
      </div>
    </section>
  );
}
