import { useState } from "react";
import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { AccountForm } from "@/components/accounts/AccountForm";
import { AccountInsights } from "@/components/accounts/AccountInsights";
import { Empty } from "@/components/ui/empty";
import type { Account, UpstreamSite } from "@/types";

export interface AccountsPanelProps {
  accounts: Account[];
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
}

export function AccountsPanel({ accounts, sites, onRefresh }: AccountsPanelProps) {
  const [detailAccount, setDetailAccount] = useState<Account | null>(null);

  return (
    <section className="accounts-panel">
      <AccountInsights accounts={accounts} onDone={onRefresh} />
      <AccountForm sites={sites} onDone={onRefresh} />
      <div className="account-grid">
        {accounts.map((account) => (
          <AccountCard
            account={account}
            key={account.id}
            onDone={onRefresh}
            onOpenDetail={() => setDetailAccount(account)}
          />
        ))}
        {!accounts.length ? <Empty message="No accounts configured yet." /> : null}
      </div>
      {detailAccount ? (
        <div className="drawer-backdrop" onClick={() => setDetailAccount(null)}>
          <aside className="detail-drawer" onClick={(event) => event.stopPropagation()}>
            <AccountDetailContent account={detailAccount} onClose={() => setDetailAccount(null)} />
          </aside>
        </div>
      ) : null}
    </section>
  );
}
