import { useEffect, useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatConfidence, formatTime } from "@/lib/format";
import type { UpstreamSite } from "@/types";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { TaskProgressView } from "@/components/ui/TaskProgressView";

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
  return enabled ? "支持" : "未知/否";
}

export function SitesPanel({ sites, onRefresh }: SitesPanelProps) {
  const [busyId, setBusyId] = useState("");
  const [message, setMessage] = useState("");
  const task = useTaskProgress();

  // 批量探测任务完成后刷新数据
  useEffect(() => {
    if (task.progress?.status === "done") {
      void onRefresh();
    }
  }, [task.progress?.status, onRefresh]);

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
      setMessage(`${site.name} 探测完成。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "探测失败。");
    } finally {
      setBusyId("");
    }
  }

  return (
    <section className="sites-panel">
      <div className="channel-summary site-summary compact-summary">
        <div>
          <span>站点</span>
          <strong>{summary.total}</strong>
        </div>
        <div>
          <span>健康</span>
          <strong>{summary.healthy}</strong>
        </div>
        <div>
          <span>可签到</span>
          <strong>{summary.checkinReady}</strong>
        </div>
        <div>
          <span>关联账号</span>
          <strong>{summary.linkedAccounts}</strong>
        </div>
      </div>

      <div className="site-bulk-actions">
        <button
          type="button"
          disabled={task.loading || task.progress?.status === "running"}
          onClick={() => void task.startTask("detect_sites")}
        >
          {task.loading || task.progress?.status === "running" ? "探测中…" : "批量探测"}
        </button>
      </div>

      <TaskProgressView
        progress={task.progress}
        loading={task.loading}
        error={task.error}
        onCancel={task.cancelTask}
        onDismiss={task.reset}
        labels={{ title: "批量识别" }}
      />

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="site-grid">
        {sites.map((site) => {
          const capabilities: Array<{ label: string; enabled?: boolean }> = [
            { label: "签到", enabled: site.supportsCheckin },
            { label: "余额", enabled: site.supportsBalance },
            { label: "模型", enabled: site.supportsModels },
            { label: "价格", enabled: site.supportsPricing },
          ];

          return (
            <article
              className={`site-card ${isUnhealthy(site.healthStatus) ? "is-unhealthy" : ""}`}
              key={site.id}
            >
              <div className="site-card-head">
                <div>
                  <span>{site.kind || "未知"}</span>
                  <strong title={site.name}>{site.name}</strong>
                </div>
                <span className={`status-pill ${isUnhealthy(site.healthStatus) ? "danger" : "neutral"}`}>
                  {site.healthStatus || "未知"}
                </span>
              </div>

              <dl className="site-addresses">
                <div>
                  <dt>基础网址</dt>
                  <dd title={site.baseUrl}>{site.baseUrl || "-"}</dd>
                </div>
                <div>
                  <dt>登录网址</dt>
                  <dd title={site.loginUrl || ""}>{site.loginUrl || "-"}</dd>
                </div>
                {site.homepageUrl ? (
                  <div>
                    <dt>主页</dt>
                    <dd title={site.homepageUrl}>{site.homepageUrl}</dd>
                  </div>
                ) : null}
              </dl>

              <div className="site-card-metrics">
                <div>
                  <span>账号</span>
                  <strong>{site.accountCount || 0}</strong>
                </div>
                <div>
                  <span>置信度</span>
                  <strong>{formatConfidence(site.detectionConfidence)}</strong>
                </div>
                <div>
                  <span>最近健康检查</span>
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
                <div className="channel-subtle">更新于 {formatTime(site.updatedAt)}</div>
              ) : null}

              <div className="site-actions">
                <button
                  disabled={busyId === site.id}
                  onClick={() => void detect(site)}
                  type="button"
                >
                  {busyId === site.id ? "探测中…" : "探测能力"}
                </button>
              </div>
            </article>
          );
        })}

        {!sites.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>暂无上游站点</strong>
            <span>请先导入 NewAPI 或 OneAPI 渠道，再在此探测站点能力。</span>
          </div>
        ) : null}
      </div>
    </section>
  );
}
