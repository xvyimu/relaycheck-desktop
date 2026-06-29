import { memo, useCallback, useEffect, useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatConfidence, formatDuration, formatTime } from "@/lib/format";
import type { NextRunItem, NavigationIntent, UpstreamSite } from "@/types";
import { useNextRuns } from "@/hooks/useNextRuns";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { TaskProgressView } from "@/components/ui/TaskProgressView";

type SitesPanelProps = {
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
};

function isUnhealthy(status: string) {
  return ["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(
    status.toLowerCase(),
  );
}

function capabilityLabel(enabled?: boolean) {
  return enabled ? "支持" : "未知/否";
}

function SitesPanelBase({ sites, onRefresh, intent }: SitesPanelProps) {
  const [busyId, setBusyId] = useState("");
  const [message, setMessage] = useState("");
  const [healthFilter, setHealthFilter] = useState<string>("all");
  const [query, setQuery] = useState("");
  const task = useTaskProgress();
  const { nextRuns } = useNextRuns();

  // 批量探测任务完成后刷新数据
  useEffect(() => {
    if (task.progress?.status === "done") {
      void onRefresh();
    }
  }, [task.progress?.status, onRefresh]);

  // Index next-runs by site name for O(1) lookup per card
  const nextRunBySite = useMemo(() => {
    const map: Record<string, NextRunItem> = {};
    for (const item of nextRuns) {
      const key = item.siteId || item.siteName;
      if (key && !map[key]) {
        map[key] = item;
      }
    }
    return map;
  }, [nextRuns]);

  const filteredSites = useMemo(() => {
    let result = sites;
    if (healthFilter === "unreachable") {
      result = result.filter((site) => isUnhealthy(site.healthStatus));
    }
    if (query.trim()) {
      const normalized = query.trim().toLowerCase();
      result = result.filter((site) =>
        [site.name, site.kind || "", site.baseUrl || "", site.loginUrl || "", site.homepageUrl || ""]
          .join(" ")
          .toLowerCase()
          .includes(normalized),
      );
    }
    return result;
  }, [sites, healthFilter, query]);

  const summary = useMemo(
    () => ({
      total: filteredSites.length,
      healthy: filteredSites.filter((site) => ["healthy", "ok", "success"].includes(site.healthStatus.toLowerCase())).length,
      checkinReady: filteredSites.filter((site) => site.supportsCheckin).length,
      linkedAccounts: filteredSites.reduce((total, site) => total + (site.accountCount || 0), 0),
    }),
    [filteredSites],
  );

  // React to navigation intent from Action Center
  useEffect(() => {
    if (!intent) return;
    if (intent.siteHealth === "unreachable") setHealthFilter("unreachable");
    if (typeof intent.query === "string") setQuery(intent.query);
  }, [intent]);

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

      <div className="site-toolbar card">
        <div className="proxy-form-grid">
          <label className="field">
            <span>搜索</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="名称、网址、类型" />
          </label>
          <label className="field">
            <span>健康状态</span>
            <select value={healthFilter} onChange={(event) => setHealthFilter(event.target.value)}>
              <option value="all">全部</option>
              <option value="unreachable">不可达/异常</option>
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" className="ghost" onClick={() => { setHealthFilter("all"); setQuery(""); }}>清除筛选</button>
        </div>
        {healthFilter === "unreachable" ? (
          <div className="channel-active-filter">
            <div>
              <strong>不可达站点筛选已启用</strong>
              <span>仅显示健康状态异常（不可达、失败、错误、过期等）的站点。</span>
            </div>
            <button type="button" className="ghost" onClick={() => { setHealthFilter("all"); setQuery(""); }}>清除</button>
          </div>
        ) : null}
      </div>

      <div className="site-grid">
        {filteredSites.map((site) => {
          const capabilities: Array<{ label: string; enabled?: boolean }> = [
            { label: "签到", enabled: site.supportsCheckin },
            { label: "余额", enabled: site.supportsBalance },
            { label: "模型", enabled: site.supportsModels },
            { label: "价格", enabled: site.supportsPricing },
          ];
          const run = nextRunBySite[site.id] || nextRunBySite[site.name];

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
                {run ? (
                  <div className="next-run-metric">
                    <span>下次签到</span>
                    <strong title={run.nextRunAt ? formatTime(run.nextRunAt) : ""}>
                      {formatDuration(run.nextRunInSeconds)}
                    </strong>
                  </div>
                ) : null}
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

        {!filteredSites.length ? (
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

export const SitesPanel = memo(SitesPanelBase);
