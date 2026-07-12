import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { api } from "@/api/client";
import { AccountsPanel } from "@/components/accounts/AccountsPanel";
import { SiteAccountMasterDetail } from "@/components/sites/SiteAccountMasterDetail";
import { formatConfidence, formatDuration, formatTime } from "@/lib/format";
import { loginDiscoverySourceLabel, normalizeLoginDiscovery, parseLoginDiscovery } from "@/lib/loginDiscovery";
import type { Account, LoginDiscovery, NextRunItem, NavigationIntent, SiteDetail, TabKey, UpstreamSite } from "@/types";
import { useNextRuns } from "@/hooks/useNextRuns";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { TaskProgressView } from "@/components/ui/TaskProgressView";
import { siteAccountsNavigationIntent } from "@/lib/navigation";

const LABELS_BATCH_DETECT = { title: "批量识别" } as const;

type SitesPanelProps = {
  sites: UpstreamSite[];
  accounts: Account[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
  onNavigate?: (tab: TabKey, intent?: Omit<NavigationIntent, "target">) => void;
};

function isUnhealthy(status: string) {
  return ["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(
    status.toLowerCase(),
  );
}

function capabilityLabel(enabled?: boolean) {
  return enabled ? "支持" : "未知/否";
}

function siteLoginDiscovery(site: UpstreamSite, detection?: SiteDetail["detection"]): LoginDiscovery | null {
  return (
    normalizeLoginDiscovery(site.loginDiscovery) ||
    parseLoginDiscovery(site.loginDiscoveryJson) ||
    normalizeLoginDiscovery(detection?.loginDiscovery) ||
    null
  );
}

function SitesPanelBase({ sites, accounts, onRefresh, intent, onNavigate }: SitesPanelProps) {
  const [busyId, setBusyId] = useState("");
  const [detailBusyId, setDetailBusyId] = useState("");
  const [detail, setDetail] = useState<SiteDetail | null>(null);
  const [message, setMessage] = useState("");
  const [healthFilter, setHealthFilter] = useState<string>("all");
  const [query, setQuery] = useState("");
  const [layoutMode, setLayoutMode] = useState<"list" | "master">("master");
  // IA-2: the standalone 账号 tab is merged here. "sites" shows the
  // site-centric master-detail; "accounts" shows the flat 全部账号 surface.
  const [subview, setSubview] = useState<"sites" | "accounts">("sites");
  const detailRequestRef = useRef(0);
  const detailCloseButtonRef = useRef<HTMLButtonElement | null>(null);
  const detailTriggerRef = useRef<HTMLButtonElement | null>(null);
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
    if (intent.accountsView === "all") {
      // Deep link into the merged 全部账号 surface.
      setSubview("accounts");
    } else if (typeof intent.upstreamSiteId === "string" && intent.upstreamSiteId.trim()) {
      // A specific site always lands on the site-centric master-detail.
      setSubview("sites");
    }
    if (intent.siteHealth === "unreachable") setHealthFilter("unreachable");
    if (typeof intent.query === "string") setQuery(intent.query);
    if (typeof intent.upstreamSiteId === "string" && intent.upstreamSiteId.trim()) {
      setLayoutMode("master");
    }
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

  async function openDetail(site: UpstreamSite, trigger?: HTMLButtonElement) {
    const requestId = detailRequestRef.current + 1;
    detailRequestRef.current = requestId;
    if (trigger) {
      detailTriggerRef.current = trigger;
    }
    setDetailBusyId(site.id);
    setMessage("");
    try {
      const nextDetail = await api<SiteDetail>(`/api/upstream-sites/${site.id}`);
      if (detailRequestRef.current === requestId) {
        setDetail(nextDetail);
      }
    } catch (error) {
      if (detailRequestRef.current === requestId) {
        setMessage(error instanceof Error ? error.message : "加载站点详情失败。");
      }
    } finally {
      if (detailRequestRef.current === requestId) {
        setDetailBusyId("");
      }
    }
  }

  const closeDetail = useCallback(() => {
    detailRequestRef.current += 1;
    const trigger = detailTriggerRef.current;
    setDetail(null);
    setDetailBusyId("");
    window.requestAnimationFrame(() => {
      trigger?.focus();
    });
  }, []);

  useEffect(() => {
    if (!detail) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") closeDetail();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [closeDetail, detail]);

  useEffect(() => {
    if (!detail) return;
    const frame = window.requestAnimationFrame(() => {
      detailCloseButtonRef.current?.focus();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [detail]);

  const detailDiscovery = detail ? siteLoginDiscovery(detail.site, detail.detection) : null;
  const detailCandidates = detailDiscovery?.candidates?.slice(0, 6) || [];

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

      <div className="segmented" role="group" aria-label="视图">
        <button
          type="button"
          className={subview === "sites" ? "active" : ""}
          onClick={() => setSubview("sites")}
        >
          按站点
        </button>
        <button
          type="button"
          className={subview === "accounts" ? "active" : ""}
          onClick={() => setSubview("accounts")}
        >
          全部账号
        </button>
      </div>

      {subview === "sites" ? (
      <div className="site-bulk-actions">
        <button
          type="button"
          disabled={task.loading || task.progress?.status === "running"}
          onClick={() => void task.startTask("detect_sites")}
        >
          {task.loading || task.progress?.status === "running" ? "探测中…" : "批量探测"}
        </button>
        <div className="segmented" role="group" aria-label="站点布局">
          <button
            type="button"
            className={layoutMode === "master" ? "active" : ""}
            onClick={() => setLayoutMode("master")}
          >
            主从
          </button>
          <button
            type="button"
            className={layoutMode === "list" ? "active" : ""}
            onClick={() => setLayoutMode("list")}
          >
            卡片
          </button>
        </div>
      </div>
      ) : null}

      <TaskProgressView
        progress={task.progress}
        loading={task.loading}
        error={task.error}
        onCancel={task.cancelTask}
        onDismiss={task.reset}
        labels={LABELS_BATCH_DETECT}
      />

      {message ? <div className="problem-hint">{message}</div> : null}

      {subview === "sites" ? (
        <>
      {layoutMode === "master" ? (
        <SiteAccountMasterDetail sites={sites} onRefresh={onRefresh} intent={intent} />
      ) : (
        <>
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
                  type="button"
                  className="ghost"
                  disabled={detailBusyId === site.id}
                  onClick={(event) => void openDetail(site, event.currentTarget)}
                >
                  {detailBusyId === site.id ? "加载中…" : "详情"}
                </button>
                {onNavigate ? (
                  <button
                    type="button"
                    className="ghost"
                    onClick={() => {
                      const next = siteAccountsNavigationIntent(site.id);
                      onNavigate(next.target, { upstreamSiteId: next.upstreamSiteId });
                    }}
                  >
                    查看账号
                  </button>
                ) : null}
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
        </>
      )}
      {detail ? (
        <div className="drawer-backdrop" role="presentation" onClick={closeDetail}>
          <aside
            className="detail-drawer detail-drawer-wide"
            role="dialog"
            aria-modal="true"
            aria-labelledby="site-detail-title"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="detail-header">
              <div>
                <span className="eyebrow">站点详情</span>
                <h2 id="site-detail-title">{detail.site.name}</h2>
                <p>{detail.site.kind || "unknown"} · {detail.site.healthStatus || "未知"}</p>
              </div>
              <div className="toolbar">
                {onNavigate ? (
                  <button
                    type="button"
                    onClick={() => {
                      const next = siteAccountsNavigationIntent(detail.site.id);
                      onNavigate(next.target, { upstreamSiteId: next.upstreamSiteId });
                    }}
                  >
                    查看账号
                  </button>
                ) : null}
                <button type="button" className="ghost" ref={detailCloseButtonRef} onClick={closeDetail}>关闭</button>
              </div>
            </div>

            <div className="detail-grid">
              <section className="detail-card">
                <h3>入口</h3>
                <div className="detail-list">
                  <div><span>基础网址</span><strong>{detail.site.baseUrl || "-"}</strong></div>
                  <div><span>主页</span><strong>{detail.site.homepageUrl || detail.detection.homepageUrl || "-"}</strong></div>
                  <div><span>登录网址</span><strong>{detail.site.loginUrl || detail.detection.loginUrl || "-"}</strong></div>
                </div>
              </section>

              <section className="detail-card">
                <h3>登录识别</h3>
                <div className="detail-metrics site-login-metrics">
                  <div>
                    <span>来源</span>
                    <strong>{loginDiscoverySourceLabel(detailDiscovery?.source || detail.site.loginUrlSource)}</strong>
                  </div>
                  <div>
                    <span>置信度</span>
                    <strong>{formatConfidence(detailDiscovery?.confidence ?? detail.site.loginUrlConfidence)}</strong>
                  </div>
                </div>
                {detailDiscovery && detailDiscovery.confidence < 0.6 ? (
                  <div className="problem-hint detail-hint">登录入口置信度偏低，建议打开站点确认一次。</div>
                ) : null}
                {detailCandidates.length ? (
                  <div className="site-login-candidates">
                    <span>候选入口</span>
                    <div>
                      {detailCandidates.map((candidate) => (
                        <code key={candidate}>{candidate}</code>
                      ))}
                    </div>
                  </div>
                ) : null}
              </section>

              <section className="detail-card">
                <h3>能力</h3>
                <div className="chips">
                  <span>签到 {capabilityLabel(detail.site.supportsCheckin)}</span>
                  <span>余额 {capabilityLabel(detail.site.supportsBalance)}</span>
                  <span>模型 {capabilityLabel(detail.site.supportsModels)}</span>
                  <span>价格 {capabilityLabel(detail.site.supportsPricing)}</span>
                </div>
              </section>

              <section className="detail-card">
                <h3>探测</h3>
                <div className="detail-list">
                  <div><span>后台类型</span><strong>{detail.detection.kind || detail.site.kind || "未知"}</strong></div>
                  <div><span>健康状态</span><strong>{detail.detection.healthStatus || detail.site.healthStatus || "未知"}</strong></div>
                  <div><span>识别置信度</span><strong>{formatConfidence(detail.detection.detectionConfidence)}</strong></div>
                  <div><span>最近检查</span><strong>{formatTime(detail.site.lastHealthCheckAt || "")}</strong></div>
                </div>
              </section>

              {detail.suggestions.length ? (
                <section className="detail-card">
                  <h3>建议</h3>
                  <div className="site-suggestions">
                    {detail.suggestions.map((suggestion) => (
                      <span key={suggestion}>{suggestion}</span>
                    ))}
                  </div>
                </section>
              ) : null}
            </div>
          </aside>
        </div>
      ) : null}
        </>
      ) : null}

      {subview === "accounts" ? (
        <AccountsPanel
          accounts={accounts}
          sites={sites}
          onRefresh={onRefresh}
          intent={intent?.target === "sites" && intent.accountsView === "all" ? intent : null}
        />
      ) : null}
    </section>
  );
}

export const SitesPanel = memo(SitesPanelBase);
