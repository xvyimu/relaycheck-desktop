import { memo, useCallback, useEffect, useMemo, useState } from "react";

import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { isProblemAccount } from "@/components/accounts/helpers";
import { DialogShell } from "@/components/ui/dialog-shell";
import { Empty } from "@/components/ui/empty";
import { useSiteAccounts } from "@/hooks/useSiteAccounts";
import type { Account, NavigationIntent, UpstreamSite } from "@/types";
import { Button } from "@/components/ui/button";

const STORAGE_KEY = "relaycheck_master_detail_site_id";

function isUnhealthy(status: string) {
  return ["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(
    status.toLowerCase(),
  );
}

function readStoredSiteId(): string {
  try {
    return (window.localStorage.getItem(STORAGE_KEY) || "").trim();
  } catch {
    return "";
  }
}

function writeStoredSiteId(siteId: string) {
  try {
    if (!siteId) {
      window.localStorage.removeItem(STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, siteId);
  } catch {
    /* ignore */
  }
}

export type SiteAccountMasterDetailProps = {
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
};

function SiteAccountMasterDetailBase({ sites, onRefresh, intent }: SiteAccountMasterDetailProps) {
  const [selectedSiteId, setSelectedSiteId] = useState<string>(() => {
    if (typeof intent?.upstreamSiteId === "string" && intent.upstreamSiteId.trim()) {
      return intent.upstreamSiteId.trim();
    }
    return readStoredSiteId();
  });
  const [query, setQuery] = useState("");
  const [healthFilter, setHealthFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [detailAccount, setDetailAccount] = useState<Account | null>(null);
  const [mobileSubview, setMobileSubview] = useState(false);

  const siteScoped = useSiteAccounts(selectedSiteId || "all");
  const { enabled: siteScopedEnabled, refresh: refreshSiteScoped } = siteScoped;

  useEffect(() => {
    if (!intent) return;
    if (typeof intent.upstreamSiteId === "string" && intent.upstreamSiteId.trim()) {
      const next = intent.upstreamSiteId.trim();
      setSelectedSiteId(next);
      writeStoredSiteId(next);
      setMobileSubview(true);
    }
    if (intent.siteHealth === "unreachable") setHealthFilter("unreachable");
    if (intent.accountStatus === "problem") setStatusFilter("problem");
  }, [intent]);

  const filteredSites = useMemo(() => {
    let result = sites;
    if (healthFilter === "unreachable") {
      result = result.filter((site) => isUnhealthy(site.healthStatus));
    }
    if (query.trim()) {
      const normalized = query.trim().toLowerCase();
      result = result.filter((site) =>
        [site.name, site.kind || "", site.baseUrl || "", site.loginUrl || ""]
          .join(" ")
          .toLowerCase()
          .includes(normalized),
      );
    }
    return result
      .slice()
      .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"));
  }, [sites, healthFilter, query]);

  const selectedSite = useMemo(
    () => sites.find((site) => site.id === selectedSiteId) || null,
    [sites, selectedSiteId],
  );

  const accounts = useMemo(() => {
    const list = siteScoped.data || [];
    if (statusFilter !== "problem") return list;
    const problems = list.filter(isProblemAccount);
    const healthy = list.filter((account) => !isProblemAccount(account));
    return [...problems, ...healthy];
  }, [siteScoped.data, statusFilter]);

  const selectSite = useCallback((siteId: string) => {
    setSelectedSiteId(siteId);
    writeStoredSiteId(siteId);
    setMobileSubview(true);
  }, []);

  const clearSelection = useCallback(() => {
    setSelectedSiteId("");
    writeStoredSiteId("");
    setMobileSubview(false);
  }, []);

  const handleDone = useCallback(async () => {
    await onRefresh();
    if (siteScopedEnabled) {
      await refreshSiteScoped();
    }
  }, [onRefresh, refreshSiteScoped, siteScopedEnabled]);

  const shellClass = [
    "master-detail",
    mobileSubview && selectedSiteId ? "master-detail-subview" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={shellClass} data-testid="site-account-master-detail">
      <aside className="master-detail-left" aria-label="站点列表">
        <div className="master-detail-toolbar card">
          <div className="master-detail-summary">
            <div>
              <span>站点</span>
              <strong>{filteredSites.length}</strong>
            </div>
            <div>
              <span>账号合计</span>
              <strong>{filteredSites.reduce((sum, site) => sum + (site.accountCount || 0), 0)}</strong>
            </div>
          </div>
          <label className="field">
            <span>搜索站点</span>
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="名称、网址、类型"
              aria-label="搜索站点"
            />
          </label>
          <label className="field">
            <span>健康</span>
            <select
              value={healthFilter}
              onChange={(event) => setHealthFilter(event.target.value)}
              aria-label="按健康状态筛选站点"
            >
              <option value="all">全部</option>
              <option value="unreachable">不可达/异常</option>
            </select>
          </label>
        </div>

        <div className="master-detail-site-list" role="listbox" aria-label="可选上游站点">
          {filteredSites.map((site) => {
            const selected = site.id === selectedSiteId;
            const unhealthy = isUnhealthy(site.healthStatus);
            return (
              <button
                key={site.id}
                type="button"
                role="option"
                aria-selected={selected}
                className={[
                  "master-detail-site-item",
                  selected ? "is-selected" : "",
                  unhealthy ? "is-unhealthy" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                onClick={() => selectSite(site.id)}
              >
                <div className="master-detail-site-head">
                  <strong title={site.name}>{site.name}</strong>
                  <span className={`status-pill ${unhealthy ? "danger" : "neutral"}`}>
                    {site.healthStatus || "未知"}
                  </span>
                </div>
                <div className="master-detail-site-meta">
                  <span>{site.kind || "unknown"}</span>
                  <span>{site.accountCount || 0} 账号</span>
                </div>
                <em title={site.baseUrl}>{site.baseUrl || "-"}</em>
              </button>
            );
          })}
          {!filteredSites.length ? (
            <div className="master-detail-empty">
              <strong>暂无站点</strong>
              <span>请先导入 NewAPI 渠道后再按站查看账号。</span>
            </div>
          ) : null}
        </div>
      </aside>

      <section className="master-detail-right" aria-label="站点账号">
        <div className="master-detail-right-head card">
          <div className="master-detail-back-row">
            <Button variant="ghost" type="button" onClick={() => setMobileSubview(false)} className="master-detail-back"
            >
              返回站点
            </Button>
            {selectedSiteId ? (
              <Button variant="ghost" type="button" onClick={clearSelection}>
                清除选中
              </Button>
            ) : null}
          </div>
          {selectedSite ? (
            <>
              <div>
                <span className="eyebrow">当前站点</span>
                <strong title={selectedSite.name}>{selectedSite.name}</strong>
                <p>
                  {selectedSite.kind || "unknown"} · {selectedSite.healthStatus || "未知"} ·{" "}
                  {accounts.length} 账号
                  {siteScoped.loading ? " · 加载中…" : ""}
                </p>
              </div>
              <label className="field master-detail-account-filter">
                <span>账号状态</span>
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  aria-label="筛选账号状态"
                >
                  <option value="all">全部</option>
                  <option value="problem">异常优先</option>
                </select>
              </label>
            </>
          ) : (
            <div className="master-detail-empty">
              <strong>选择左侧站点</strong>
              <span>选中后将通过服务端按站点加载账号，不拉取全量列表。</span>
            </div>
          )}
          {siteScoped.error ? <div className="error">{siteScoped.error}</div> : null}
        </div>

        {selectedSiteId ? (
          <div className="master-detail-account-grid">
            {accounts.map((account) => (
              <AccountCard
                key={account.id}
                account={account}
                onDone={handleDone}
                onOpenDetail={() => setDetailAccount(account)}
              />
            ))}
            {!accounts.length ? (
              <Empty
                message={
                  siteScoped.loading
                    ? "正在按站点加载账号…"
                    : "该站点下暂无账号。"
                }
              />
            ) : null}
          </div>
        ) : null}
      </section>

      <DialogShell
        open={Boolean(detailAccount)}
        onClose={() => setDetailAccount(null)}
        variant="panel"
        ariaLabel={detailAccount ? `账号详情 ${detailAccount.displayName || detailAccount.id}` : "账号详情"}
        initialFocusSelector=".detail-header .ghost, .detail-header button, button.ghost"
      >
        {detailAccount ? (
          <AccountDetailContent account={detailAccount} onClose={() => setDetailAccount(null)} />
        ) : null}
      </DialogShell>
    </div>
  );
}

export const SiteAccountMasterDetail = memo(SiteAccountMasterDetailBase);
