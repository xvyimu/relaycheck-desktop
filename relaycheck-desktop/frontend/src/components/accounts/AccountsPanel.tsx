import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { AccountForm } from "@/components/accounts/AccountForm";
import { AccountInsights } from "@/components/accounts/AccountInsights";
import { isProblemAccount } from "@/components/accounts/helpers";
import { Empty } from "@/components/ui/empty";
import { useSiteAccounts } from "@/hooks/useSiteAccounts";
import type { Account, NavigationIntent, UpstreamSite } from "@/types";

export interface AccountsPanelProps {
  accounts: Account[];
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
}

function AccountsPanelBase({ accounts, sites, onRefresh, intent }: AccountsPanelProps) {
  const [detailAccount, setDetailAccount] = useState<Account | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>(() =>
    intent?.accountStatus === "problem" ? "problem" : "all",
  );
  const [siteFilter, setSiteFilter] = useState<string>(() => {
    if (typeof intent?.upstreamSiteId === "string") {
      const next = intent.upstreamSiteId.trim();
      return next || "all";
    }
    return "all";
  });
  const [query, setQuery] = useState(() =>
    typeof intent?.query === "string" ? intent.query : "",
  );

  // S3: server-side site filter — only hits /api/accounts when a site is selected.
  const siteScoped = useSiteAccounts(siteFilter);

  // React to navigation intent from Action Center / sites "查看账号"
  useEffect(() => {
    if (!intent) return;
    if (intent.accountStatus === "problem") setStatusFilter("problem");
    else if (intent.accountStatus === "all") setStatusFilter("all");
    if (typeof intent.upstreamSiteId === "string") {
      const nextSite = intent.upstreamSiteId.trim();
      setSiteFilter(nextSite || "all");
    }
    if (typeof intent.query === "string") setQuery(intent.query);
  }, [intent]);

  const handleDone = useCallback(async () => {
    await onRefresh();
    if (siteScoped.enabled) {
      await siteScoped.refresh();
    }
  }, [onRefresh, siteScoped.enabled, siteScoped.refresh]);

  const siteOptions = useMemo(() => {
    const counts = new Map<string, number>();
    for (const account of accounts) {
      counts.set(account.upstreamSiteId, (counts.get(account.upstreamSiteId) || 0) + 1);
    }
    return sites
      .filter((site) => counts.has(site.id) || site.accountCount > 0 || site.id === siteFilter)
      .slice()
      .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"));
  }, [accounts, sites, siteFilter]);

  // Base list: server payload when site-scoped; inventory + client filter as fallback / "all".
  const baseAccounts = useMemo(() => {
    if (siteFilter === "all") return accounts;
    if (siteScoped.data) return siteScoped.data;
    // Optimistic client filter while server loads (S1 path / SSR tests).
    return accounts.filter((account) => account.upstreamSiteId === siteFilter);
  }, [accounts, siteFilter, siteScoped.data]);

  const filteredAccounts = useMemo(() => {
    let result = baseAccounts;
    if (statusFilter === "problem") {
      const problems = result.filter(isProblemAccount);
      const healthy = result.filter((a) => !isProblemAccount(a));
      result = [...problems, ...healthy];
    }
    if (query.trim()) {
      const normalized = query.trim().toLowerCase();
      result = result.filter((a) =>
        [a.displayName, a.email || "", a.username || "", a.upstreamSiteName || "", a.loginStatus || ""]
          .join(" ")
          .toLowerCase()
          .includes(normalized),
      );
    }
    return result;
  }, [baseAccounts, statusFilter, query]);

  function clearFilters() {
    setStatusFilter("all");
    setSiteFilter("all");
    setQuery("");
  }

  const hasActiveFilter = statusFilter !== "all" || siteFilter !== "all" || query.trim() !== "";
  const selectedSiteName =
    siteFilter === "all" ? "" : siteOptions.find((site) => site.id === siteFilter)?.name || siteFilter;

  useEffect(() => {
    if (!detailAccount) return;
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setDetailAccount(null);
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [detailAccount]);

  return (
    <section className="accounts-panel">
      <AccountInsights accounts={accounts} onDone={handleDone} />
      <AccountForm sites={sites} onDone={handleDone} />
      <div className="account-toolbar card">
        <div className="channel-summary compact-summary">
          <div><span>全部</span><strong>{accounts.length}</strong></div>
          <div><span>异常</span><strong>{accounts.filter(isProblemAccount).length}</strong></div>
          <div><span>可见</span><strong>{filteredAccounts.length}</strong></div>
        </div>
        <div className="account-filter-grid">
          <label className="field">
            <span>站点</span>
            <select
              value={siteFilter}
              onChange={(event) => setSiteFilter(event.target.value)}
              aria-label="按上游站点筛选账号"
            >
              <option value="all">全部站点</option>
              {siteOptions.map((site) => (
                <option key={site.id} value={site.id}>
                  {site.name}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span>状态</span>
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
              <option value="all">全部</option>
              <option value="problem">异常账号</option>
            </select>
          </label>
          <label className="field">
            <span>搜索</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="账号名、邮箱、站点" />
          </label>
        </div>
        <div className="toolbar">
          <button type="button" className="ghost" onClick={clearFilters} disabled={!hasActiveFilter}>
            清除筛选
          </button>
        </div>
        {statusFilter === "problem" || siteFilter !== "all" ? (
          <div className="channel-active-filter">
            <div>
              <strong>
                {[
                  siteFilter !== "all" ? `站点：${selectedSiteName || siteFilter}` : "",
                  statusFilter === "problem" ? "异常账号优先" : "",
                  siteScoped.enabled && siteScoped.loading ? "服务端筛选中…" : "",
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </strong>
              <span>
                {siteFilter !== "all" && statusFilter === "problem"
                  ? "仅显示该站点账号，异常排在最前。"
                  : siteFilter !== "all"
                    ? siteScoped.enabled
                      ? "服务端按上游站点过滤账号列表。"
                      : "仅显示该上游站点下的账号。"
                    : "异常账号排在最前，包括登录异常和签到异常的账号。"}
              </span>
              {siteScoped.error ? <span className="danger-text">{siteScoped.error}</span> : null}
            </div>
            <button type="button" className="ghost" onClick={clearFilters}>清除</button>
          </div>
        ) : null}
      </div>
      <div className="account-grid">
        {filteredAccounts.map((account) => (
          <AccountCard
            account={account}
            key={account.id}
            onDone={handleDone}
            onOpenDetail={() => setDetailAccount(account)}
          />
        ))}
        {!filteredAccounts.length ? (
          <Empty
            message={
              siteScoped.enabled && siteScoped.loading
                ? "正在按站点加载账号…"
                : "No accounts match current filters."
            }
          />
        ) : null}
      </div>
      {detailAccount ? (
        <div className="drawer-backdrop" role="presentation" onClick={() => setDetailAccount(null)}>
          <aside className="detail-drawer" onClick={(event) => event.stopPropagation()}>
            <AccountDetailContent account={detailAccount} onClose={() => setDetailAccount(null)} />
          </aside>
        </div>
      ) : null}
    </section>
  );
}

export const AccountsPanel = memo(AccountsPanelBase);
