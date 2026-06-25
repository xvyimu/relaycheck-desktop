import { useEffect, useState } from "react";
import { api } from "@/api/client";
import type { VersionCheckResult } from "@/types";

const DISMISS_KEY = "rc.updateBanner.dismissedVersion";

/**
 * UpdateBanner polls /api/system/version-check on mount and, when a newer
 * version is available, renders a dismissible banner at the top of the
 * dashboard. Dismissing a version records it in localStorage so the banner
 * stays quiet until an even newer version is published.
 */
export function UpdateBanner() {
  const [result, setResult] = useState<VersionCheckResult | null>(null);
  const [dismissed, setDismissed] = useState<string>(() => {
    if (typeof window === "undefined") return "";
    return window.localStorage.getItem(DISMISS_KEY) || "";
  });

  useEffect(() => {
    let active = true;
    api<VersionCheckResult>("/api/system/version-check")
      .then((data) => {
        if (active) setResult(data);
      })
      .catch(() => {
        // Version check is best-effort; never block the dashboard on failure.
      });
    return () => {
      active = false;
    };
  }, []);

  if (!result || !result.updateAvailable) return null;
  if (dismissed && dismissed === (result.latestVersion || "")) return null;

  const latest = result.latestVersion || "新版本";

  const handleDismiss = () => {
    const version = result.latestVersion || "";
    if (version) {
      window.localStorage.setItem(DISMISS_KEY, version);
      setDismissed(version);
    }
  };

  return (
    <div className="update-banner" role="status" aria-live="polite">
      <div className="update-banner-content">
        <span className="update-banner-icon" aria-hidden="true">
          ↑
        </span>
        <div className="update-banner-text">
          <strong>发现新版本 {latest}</strong>
          <span>
            当前版本 {result.currentVersion}，建议尽快更新到最新版本。
            {result.releaseNotes ? ` ${result.releaseNotes}` : ""}
          </span>
        </div>
      </div>
      <div className="update-banner-actions">
        {result.releaseUrl ? (
          <a
            className="update-banner-link"
            href={result.releaseUrl}
            target="_blank"
            rel="noreferrer"
          >
            查看更新
          </a>
        ) : null}
        <button type="button" className="ghost" onClick={handleDismiss}>
          稍后提醒
        </button>
      </div>
    </div>
  );
}
