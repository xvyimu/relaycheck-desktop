import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { AccountForm } from "@/components/accounts/AccountForm";
import { AccountInsights } from "@/components/accounts/AccountInsights";
import { BulkReloginWizard } from "@/components/accounts/BulkReloginWizard";
import { DialogShell } from "@/components/ui/dialog-shell";
import { Empty } from "@/components/ui/empty";
import { useAccountsPage } from "@/hooks/useAccountsPage";
import { useDebouncedValue } from "@/hooks/useDebouncedValue";
import type { Account, NavigationIntent, UpstreamSite } from "@/types";
import { Button } from "@/components/ui/button";

export interface AccountsPanelProps {
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
}

function AccountsPanelBase({ sites, onRefresh, intent }: AccountsPanelProps) {
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
  const [queryInput, setQueryInput] = useState(() => (typeof intent?.query === "string" ? intent.query : ""));
  const [isComposing, setIsComposing] = useState(false);
  const query = useDebouncedValue(queryInput, 250, !isComposing);

  // Server-side paginated accounts
  const pagination = useAccountsPage({
    limit: 50,
    query,
    status: statusFilter,
    upstreamSiteId: siteFilter,
  });

  // React to navigation intent from Action Center / sites "查看账号"
  useEffect(() => {
    if (!intent) return;
    if (intent.accountStatus === "problem") setStatusFilter("problem");
    else if (intent.accountStatus === "all") setStatusFilter("all");
    if (typeof intent.upstreamSiteId === "string") {
      const nextSite = intent.upstreamSiteId.trim();
      setSiteFilter(nextSite || "all");
    }
    if (typeof intent.query === "string") setQueryInput(intent.query);
  }, [intent]);

  const handleDone = useCallback(async () => {
    await pagination.refresh();
    await onRefresh();
  }, [onRefresh, pagination]);

  const siteOptions = useMemo(() => {
    return sites.slice().sort((left, right) => left.name.localeCompare(right.name, "zh-CN"));
  }, [sites]);

  const hasActiveFilter = statusFilter !== "all" || siteFilter !== "all" || queryInput.trim() !== "";
  const selectedSiteName =
    siteFilter === "all" ? "" : siteOptions.find((site) => site.id === siteFilter)?.name || siteFilter;

  function clearFilters() {
    setStatusFilter("all");
    setSiteFilter("all");
    setQueryInput("");
  }

  const pageAccounts = pagination.page.items;
  const total = pagination.page.total;

  return (
    <section className="accounts-panel">
      <AccountInsights accounts={pageAccounts} onDone={handleDone} />
      <BulkReloginWizard accounts={pageAccounts} onDone={handleDone} />
      <AccountForm sites={sites} onDone={handleDone} />
      <div className="account-toolbar card">
        <div className="channel-summary compact-summary">
          <div>
            <span>总数</span>
            <strong>{pagination.page.accountTotal}</strong>
          </div>
          <div>
            <span>异常</span>
            <strong>{pagination.page.problemTotal}</strong>
          </div>
          <div>
            <span>过滤后</span>
            <strong>{total}</strong>
          </div>
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
            <input
              value={queryInput}
              onChange={(event) => setQueryInput(event.target.value)}
              onCompositionStart={() => setIsComposing(true)}
              onCompositionEnd={(event) => {
                setQueryInput(event.currentTarget.value);
                setIsComposing(false);
              }}
              placeholder="账号名、邮箱、站点"
            />
          </label>
        </div>
        <div className="toolbar">
          <Button variant="ghost" type="button" onClick={clearFilters} disabled={!hasActiveFilter}>
            清除筛选
          </Button>
        </div>
        {statusFilter === "problem" || siteFilter !== "all" ? (
          <div className="channel-active-filter">
            <div>
              <strong>
                {[
                  siteFilter !== "all" ? `站点：${selectedSiteName || siteFilter}` : "",
                  statusFilter === "problem" ? "异常账号优先" : "",
                  pagination.loading ? "加载中…" : "",
                ]
                  .filter(Boolean)
                  .join(" · ")}
              </strong>
              <span>服务端分页查询，仅加载当前页数据。</span>
            </div>
            <Button variant="ghost" type="button" onClick={clearFilters}>
              清除
            </Button>
          </div>
        ) : null}
      </div>
      <div className="account-grid">
        {pagination.loading && !pageAccounts.length ? <Empty message="正在加载账号…" /> : null}
        {!pagination.loading &&
          pageAccounts.map((account) => (
            <AccountCard
              account={account}
              key={account.id}
              onDone={handleDone}
              onOpenDetail={() => setDetailAccount(account)}
            />
          ))}
        {!pagination.loading && !pageAccounts.length ? <Empty message="没有匹配的账号。请调整筛选条件。" /> : null}
        {pagination.error ? <Empty message={`加载失败：${pagination.error}`} /> : null}
      </div>
      {total > 0 ? (
        <div className="pagination-bar">
          <Button variant="ghost" type="button" disabled={!pagination.hasPrev} onClick={pagination.goPrev}>
            上一页
          </Button>
          <span>
            共 {total} 个账号
            {pagination.hasNext ? " · 后页可用" : " · 最后一页"}
          </span>
          <Button variant="ghost" type="button" disabled={!pagination.hasNext} onClick={pagination.goNext}>
            下一页
          </Button>
        </div>
      ) : null}
      <DialogShell
        open={Boolean(detailAccount)}
        onClose={() => setDetailAccount(null)}
        variant="panel"
        ariaLabel={detailAccount ? `账号详情 ${detailAccount.displayName || detailAccount.id}` : "账号详情"}
        initialFocusSelector=".detail-header .ghost, .detail-header button, button.ghost"
      >
        {detailAccount ? <AccountDetailContent account={detailAccount} onClose={() => setDetailAccount(null)} /> : null}
      </DialogShell>
    </section>
  );
}

export const AccountsPanel = memo(AccountsPanelBase);
