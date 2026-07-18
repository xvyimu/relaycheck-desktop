import { useEffect, useState } from "react";
import { systemApi } from "@/api/system";
import type { VersionCheckResult } from "@/types";
import { Button } from "@/components/ui/button";
import { safeExternalUrl } from "@/lib/safeExternalUrl";

const DISMISS_KEY = "rc.updateBanner.dismissedVersion";

/**
 * UpdateBanner 挂载时通过 systemApi 拉取版本检查；
 * 有新版本时展示可关闭横幅。关闭记录进 localStorage。
 */
export function UpdateBanner() {
  const [result, setResult] = useState<VersionCheckResult | null>(null);
  const [dismissed, setDismissed] = useState<string>(() => {
    if (typeof window === "undefined") return "";
    return window.localStorage.getItem(DISMISS_KEY) || "";
  });

  useEffect(() => {
    let active = true;
    systemApi
      .versionCheck()
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
  const releaseHref = safeExternalUrl(result.releaseUrl);

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
        {releaseHref ? (
          <a className="update-banner-link" href={releaseHref} target="_blank" rel="noreferrer noopener">
            查看更新
          </a>
        ) : null}
        <Button variant="ghost" type="button" onClick={handleDismiss}>
          稍后提醒
        </Button>
      </div>
    </div>
  );
}
