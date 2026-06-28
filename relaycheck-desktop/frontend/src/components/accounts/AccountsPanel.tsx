import { useEffect, useMemo, useState } from "react";
import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { AccountForm } from "@/components/accounts/AccountForm";
import { AccountInsights } from "@/components/accounts/AccountInsights";
import { isProblemAccount } from "@/components/accounts/helpers";
import { Empty } from "@/components/ui/empty";
import type { Account, NavigationIntent, UpstreamSite } from "@/types";

export interface AccountsPanelProps {
  accounts: Account[];
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
}

export function AccountsPanel({ accounts, sites, onRefresh, intent }: AccountsPanelProps) {
  const [detailAccount, setDetailAccount] = useState<Account | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [query, setQuery] = useState("");

  // React to navigation intent from Action Center
  useEffect(() => {
    if (!intent) return;
    if (intent.accountStatus === "problem") setStatusFilter("problem");
    if (typeof intent.query === "string") setQuery(intent.query);
  }, [intent]);

  const filteredAccounts = useMemo(() => {
    let result = accounts;
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
  }, [accounts, statusFilter, query]);

  function clearFilters() {
    setStatusFilter("all");
    setQuery("");
  }

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
      <AccountInsights accounts={accounts} onDone={onRefresh} />
      <AccountForm sites={sites} onDone={onRefresh} />
      <div className="account-toolbar card">
        <div className="channel-summary compact-summary">
          <div><span>全部</span><strong>{accounts.length}</strong></div>
          <div><span>异常</span><strong>{accounts.filter(isProblemAccount).length}</strong></div>
          <div><span>可见</span><strong>{filteredAccounts.length}</strong></div>
        </div>
        <div className="proxy-form-grid">
          <label className="field">
            <span>搜索</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="账号名、邮箱、站点" />
          </label>
          <label className="field">
            <span>状态</span>
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
              <option value="all">全部</option>
              <option value="problem">异常账号</option>
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" className="ghost" onClick={clearFilters}>清除筛选</button>
        </div>
        {statusFilter === "problem" ? (
          <div className="channel-active-filter">
            <div>
              <strong>异常账号筛选已启用</strong>
              <span>异常账号排在最前，包括登录异常和签到异常的账号。</span>
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
            onDone={onRefresh}
            onOpenDetail={() => setDetailAccount(account)}
          />
        ))}
        {!filteredAccounts.length ? <Empty message="No accounts match current filters." /> : null}
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
