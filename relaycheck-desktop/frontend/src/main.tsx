import { StrictMode, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { api } from "@/api/client";
import { AccountCard } from "@/components/accounts/AccountCard";
import { AccountDetailContent } from "@/components/accounts/AccountDetailContent";
import { AccountForm } from "@/components/accounts/AccountForm";
import { AccountInsights } from "@/components/accounts/AccountInsights";
import { CheckinsPanel } from "@/components/checkins/CheckinsPanel";
import { ChannelTable } from "@/components/channels/ChannelTable";
import { HubRadar } from "@/components/dashboard/HubRadar";
import { NotificationsPanel } from "@/components/notifications/NotificationsPanel";
import { Settings as SettingsPanel } from "@/components/settings/Settings";
import { SitesPanel } from "@/components/sites/SitesPanel";
import { useChannelActions } from "@/hooks/useChannelActions";
import { useChannelFilters } from "@/hooks/useChannelFilters";
import { formatTime } from "@/lib/format";
import type {
  Account,
  ActionCenter,
  CheckinStatus,
  ImportedChannel,
  ModelOverview,
  ModelPricingOverview,
  NotificationItem,
  SessionPayload,
  StatusPayload,
  SystemDiagnostics,
  TabKey,
  UpstreamSite,
  UsageOverview,
} from "@/types";
import "./recovery.css";

type Tab = "dashboard" | "channels" | "sites" | "accounts" | "checkins" | "notifications" | "settings";

const tabs: Array<{ key: Tab; label: string }> = [
  { key: "dashboard", label: "Dashboard" },
  { key: "channels", label: "Channels" },
  { key: "sites", label: "Sites" },
  { key: "accounts", label: "Accounts" },
  { key: "checkins", label: "Check-ins" },
  { key: "notifications", label: "Notifications" },
  { key: "settings", label: "Settings" },
];

function numberValue(value: number | undefined) {
  return typeof value === "number" ? value.toLocaleString() : "0";
}

function statusTone(value?: string) {
  const normalized = (value || "unknown").toLowerCase();
  if (["success", "ok", "healthy", "active", "valid", "enabled"].includes(normalized)) return "good";
  if (["failed", "error", "danger", "invalid", "expired", "unreachable"].includes(normalized)) return "bad";
  if (["warning", "missing", "archived", "unknown", "unchecked"].includes(normalized)) return "warn";
  return "neutral";
}

function Badge({ value }: { value?: string }) {
  const label = value || "unknown";
  return <span className={`badge ${statusTone(label)}`}>{label}</span>;
}

function App() {
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);
  const [tab, setTab] = useState<Tab>("dashboard");
  const [error, setError] = useState("");
  const [status, setStatus] = useState<StatusPayload | null>(null);
  const [channels, setChannels] = useState<ImportedChannel[]>([]);
  const [sites, setSites] = useState<UpstreamSite[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [checkins, setCheckins] = useState<CheckinStatus | null>(null);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [diagnostics, setDiagnostics] = useState<SystemDiagnostics | null>(null);
  const [actionCenter, setActionCenter] = useState<ActionCenter | null>(null);
  const [modelOverview, setModelOverview] = useState<ModelOverview | null>(null);
  const [pricingOverview, setPricingOverview] = useState<ModelPricingOverview | null>(null);
  const [usageOverview, setUsageOverview] = useState<UsageOverview | null>(null);

  async function loadSession() {
    setLoading(true);
    setError("");
    try {
      const session = await api<SessionPayload>("/api/auth/session");
      setAuthenticated(session.authenticated);
      if (session.authenticated) {
        await loadData();
      }
    } catch (err) {
      setAuthenticated(false);
      setError(err instanceof Error ? err.message : "Failed to load session");
    } finally {
      setLoading(false);
    }
  }

  async function loadData() {
    const [
      nextStatus,
      nextChannels,
      nextSites,
      nextAccounts,
      nextCheckins,
      nextNotifications,
      nextDiagnostics,
      nextActionCenter,
      nextModelOverview,
      nextPricingOverview,
      nextUsageOverview,
    ] = await Promise.all([
      api<StatusPayload>("/api/system/status"),
      api<ImportedChannel[]>("/api/channels"),
      api<UpstreamSite[]>("/api/upstream-sites"),
      api<Account[]>("/api/accounts"),
      api<CheckinStatus>("/api/checkins/status"),
      api<NotificationItem[]>("/api/notifications"),
      api<SystemDiagnostics>("/api/system/diagnostics"),
      api<ActionCenter>("/api/system/action-center"),
      api<ModelOverview>("/api/models/overview"),
      api<ModelPricingOverview>("/api/models/pricing"),
      api<UsageOverview>("/api/usage/overview"),
    ]);
    setStatus(nextStatus);
    setChannels(nextChannels);
    setSites(nextSites);
    setAccounts(nextAccounts);
    setCheckins(nextCheckins);
    setNotifications(nextNotifications);
    setDiagnostics(nextDiagnostics);
    setActionCenter(nextActionCenter);
    setModelOverview(nextModelOverview);
    setPricingOverview(nextPricingOverview);
    setUsageOverview(nextUsageOverview);
  }

  useEffect(() => {
    void loadSession();
  }, []);

  if (loading) {
    return (
      <main className="center-screen">
        <div className="loading-card">Starting RelayCheck Desktop...</div>
      </main>
    );
  }

  if (!authenticated) {
    return <Login error={error} onLoggedIn={loadSession} />;
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">R</span>
          <div>
            <strong>RelayCheck</strong>
            <small>Recovery Console</small>
          </div>
        </div>
        <nav>
          {tabs.map((item) => (
            <button key={item.key} className={tab === item.key ? "active" : ""} onClick={() => setTab(item.key)}>
              {item.label}
            </button>
          ))}
        </nav>
      </aside>

      <main className="main-panel">
        <header className="topbar">
          <div>
            <p className="eyebrow">P0 baseline restored</p>
            <h1>{tabs.find((item) => item.key === tab)?.label}</h1>
          </div>
          <div className="topbar-actions">
            <button onClick={() => void loadData()}>Refresh</button>
            <button
              className="ghost"
              onClick={async () => {
                await api("/api/auth/logout", { method: "POST" });
                setAuthenticated(false);
              }}
            >
              Log out
            </button>
          </div>
        </header>
        {error ? <div className="notice error">{error}</div> : null}
        {tab === "dashboard" ? (
          <Dashboard
            status={status}
            channels={channels}
            sites={sites}
            accounts={accounts}
            checkins={checkins}
            notifications={notifications}
            diagnostics={diagnostics}
            actionCenter={actionCenter}
            modelOverview={modelOverview}
            pricingOverview={pricingOverview}
            usageOverview={usageOverview}
            onNavigate={(nextTab) => {
              if (tabs.some((item) => item.key === nextTab)) setTab(nextTab as Tab);
            }}
            onRefresh={loadData}
          />
        ) : null}
        {tab === "channels" ? <Channels onRefresh={loadData} /> : null}
        {tab === "sites" ? <SitesPanel sites={sites} onRefresh={loadData} /> : null}
        {tab === "accounts" ? <Accounts accounts={accounts} sites={sites} onRefresh={loadData} /> : null}
        {tab === "checkins" ? <CheckinsPanel checkins={checkins} onRefresh={loadData} /> : null}
        {tab === "notifications" ? <NotificationsPanel items={notifications} onRefresh={loadData} /> : null}
        {tab === "settings" ? (
          status ? <SettingsPanel status={status} onDone={loadData} /> : <Empty message="Loading settings..." />
        ) : null}
      </main>
    </div>
  );
}

function Login({ error, onLoggedIn }: { error: string; onLoggedIn: () => Promise<void> }) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [localError, setLocalError] = useState(error);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setLocalError("");
    try {
      await api("/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      await onLoggedIn();
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="center-screen">
      <form className="login-card" onSubmit={submit}>
        <div>
          <p className="eyebrow">Local console</p>
          <h1>RelayCheck Desktop</h1>
          <p>Use the local administrator account to continue.</p>
        </div>
        <label>
          Username
          <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
        </label>
        <label>
          Password
          <input
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            type="password"
            autoComplete="current-password"
            placeholder="Enter local password"
          />
        </label>
        {localError ? <div className="notice error">{localError}</div> : null}
        <button disabled={busy}>{busy ? "Logging in..." : "Log in"}</button>
      </form>
    </main>
  );
}

function Dashboard({
  status,
  channels,
  sites,
  accounts,
  checkins,
  notifications,
  diagnostics,
  actionCenter,
  modelOverview,
  pricingOverview,
  usageOverview,
  onNavigate,
  onRefresh,
}: {
  status: StatusPayload | null;
  channels: ImportedChannel[];
  sites: UpstreamSite[];
  accounts: Account[];
  checkins: CheckinStatus | null;
  notifications: NotificationItem[];
  diagnostics: SystemDiagnostics | null;
  actionCenter: ActionCenter | null;
  modelOverview: ModelOverview | null;
  pricingOverview: ModelPricingOverview | null;
  usageOverview: UsageOverview | null;
  onNavigate: (tab: TabKey) => void;
  onRefresh: () => Promise<void>;
}) {
  const problemChannels = channels.filter((item) => item.sourceSyncStatus === "missing" || item.upstreamKind === "unknown").length;
  const problemAccounts = accounts.filter((item) => ["expired", "invalid", "failed"].includes((item.loginStatus || "").toLowerCase())).length;
  const unread = notifications.filter((item) => !item.read).length;
  const schedulerJobs = status?.scheduler?.jobs || [];

  return (
    <>
      {status ? (
        <HubRadar
          status={status}
          diagnostics={diagnostics}
          actionCenter={actionCenter}
          modelOverview={modelOverview}
          pricingOverview={pricingOverview}
          usageOverview={usageOverview}
          onNavigate={(nextTab) => onNavigate(nextTab)}
          onRefresh={() => void onRefresh()}
        />
      ) : null}
      <section className="metric-grid">
        <Metric title="Local NewAPI" value={status?.summary.localNewApiCount} />
        <Metric title="Channels" value={status?.summary.importedChannelCount ?? channels.length} />
        <Metric title="Identified" value={status?.summary.identifiedChannelCount} />
        <Metric title="Accounts" value={status?.summary.accountCount ?? accounts.length} />
        <Metric title="Unread" value={status?.summary.unreadNotifications ?? unread} />
      </section>
      <section className="card-grid">
        <Card title="System">
          <dl className="kv">
            <dt>Product</dt>
            <dd>{status?.productName || "RelayCheck Desktop"}</dd>
            <dt>Version</dt>
            <dd>{status?.productVersion || "unknown"}</dd>
            <dt>Runtime</dt>
            <dd>{status ? `${status.bindAddress}:${status.port}` : "unknown"}</dd>
            <dt>Diagnostics</dt>
            <dd>{status?.lastDiagnostics?.overall || "unknown"}</dd>
          </dl>
        </Card>
        <Card title="Operations">
          <div className="stack">
            <Row label="Channels needing review" value={problemChannels} />
            <Row label="Accounts needing review" value={problemAccounts} />
            <Row label="Check-ins due today" value={checkins?.today.dueAccounts ?? 0} />
            <Row label="Failed check-ins today" value={checkins?.today.failedCount ?? 0} />
          </div>
        </Card>
        <Card title="Scheduler">
          {schedulerJobs.length ? (
            <div className="stack">
              {schedulerJobs.slice(0, 4).map((job) => (
                <div className="list-row" key={job.key}>
                  <div>
                    <strong>{job.label}</strong>
                    <span>{job.nextRunAt ? `Next: ${formatTime(job.nextRunAt)}` : job.lastError || "No next run"}</span>
                  </div>
                  <Badge value={job.status} />
                </div>
              ))}
            </div>
          ) : (
            <Empty message="No scheduler data yet." />
          )}
        </Card>
      </section>
    </>
  );
}

function Channels({ onRefresh }: { onRefresh: () => Promise<void> }) {
  const actions = useChannelActions();
  const filters = useChannelFilters(actions.channels, actions.accounts);

  useEffect(() => {
    void actions.refresh();
  }, [actions.refresh]);

  async function refreshAll() {
    await actions.refresh();
    await onRefresh();
  }

  return (
    <section className="channels-panel">
      <div className="channel-toolbar card">
        <div className="channel-summary compact-summary">
          <div><span>Visible</span><strong>{filters.visibleChannels.length}</strong></div>
          <div><span>Identified</span><strong>{filters.identifiedCount}</strong></div>
          <div><span>Target relays</span><strong>{filters.targetRelayCount}</strong></div>
          <div><span>Source missing</span><strong>{filters.sourceMissingCount}</strong></div>
        </div>
        <div className="proxy-form-grid">
          <label className="field">
            <span>Search</span>
            <input value={filters.query} onChange={(event) => filters.setQuery(event.target.value)} placeholder="Name, URL, model, account" />
          </label>
          <label className="field">
            <span>Source status</span>
            <select value={filters.sourceStatusFilter} onChange={(event) => filters.setSourceStatusFilter(event.target.value)}>
              <option value="not_archived">Active + missing</option>
              <option value="all">All</option>
              <option value="active">Active</option>
              <option value="missing">Missing</option>
              <option value="archived">Archived</option>
            </select>
          </label>
          <label className="field">
            <span>Backend kind</span>
            <select value={filters.kindFilter} onChange={(event) => filters.setKindFilter(event.target.value)}>
              <option value="target_relay">Target relays</option>
              <option value="all">All kinds</option>
              {filters.kindOptions.map((kind) => (
                <option key={kind} value={kind}>{kind}</option>
              ))}
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" onClick={() => void actions.syncChannelModels()} disabled={actions.modelSyncing}>
            {actions.modelSyncing ? "Syncing..." : "Sync models"}
          </button>
          <button type="button" className="ghost" onClick={() => void refreshAll()}>Refresh</button>
          <button type="button" className="ghost" onClick={filters.clearFilters}>Clear filters</button>
        </div>
        {actions.message ? <div className="note">{actions.message}</div> : null}
      </div>
      <ChannelTable
        channels={actions.channels}
        loaded={actions.loaded}
        message={actions.message}
        onSetDrawer={actions.setDrawer}
        onSetMessage={actions.setMessage}
        onRefresh={refreshAll}
        onUpdateSourceStatus={actions.updateChannelSourceStatus}
        filters={filters}
      />
      {actions.drawer?.kind === "channel" ? (
        <div className="drawer-backdrop" onClick={() => actions.setDrawer(null)}>
          <aside className="detail-drawer" onClick={(event) => event.stopPropagation()}>
            <div className="detail-header">
              <div>
                <span className="eyebrow">Channel detail</span>
                <h2>{actions.drawer.channel.name}</h2>
              </div>
              <button className="ghost" onClick={() => actions.setDrawer(null)}>Close</button>
            </div>
            <div className="detail-grid">
              <section className="detail-card">
                <h3>Runtime</h3>
                <div className="detail-list">
                  <div><span>Base URL</span><strong>{actions.drawer.channel.baseUrl || "-"}</strong></div>
                  <div><span>Kind</span><strong>{actions.drawer.channel.upstreamKind || "unknown"}</strong></div>
                  <div><span>Models</span><strong>{actions.drawer.channel.modelCount || 0}</strong></div>
                  <div><span>Source</span><strong>{actions.drawer.channel.sourceSyncStatus || "active"}</strong></div>
                </div>
              </section>
              <section className="detail-card">
                <h3>Capabilities</h3>
                <div className="chips">
                  <span>Check-in {actions.drawer.channel.supportsCheckin ? "yes" : "unknown/no"}</span>
                  <span>Balance {actions.drawer.channel.supportsBalance ? "yes" : "unknown/no"}</span>
                  <span>Models {actions.drawer.channel.supportsModels ? "yes" : "unknown/no"}</span>
                  <span>Pricing {actions.drawer.channel.supportsPricing ? "yes" : "unknown/no"}</span>
                </div>
              </section>
            </div>
          </aside>
        </div>
      ) : null}
    </section>
  );
}

function Accounts({
  accounts,
  sites,
  onRefresh,
}: {
  accounts: Account[];
  sites: UpstreamSite[];
  onRefresh: () => Promise<void>;
}) {
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

function Metric({ title, value }: { title: string; value?: number }) {
  return (
    <div className="metric-card">
      <span>{title}</span>
      <strong>{numberValue(value)}</strong>
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="card">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

function Row({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="kv-row">
      <span>{label}</span>
      <strong>{typeof value === "number" ? value.toLocaleString() : value}</strong>
    </div>
  );
}

function Empty({ message }: { message: string }) {
  return <div className="empty">{message}</div>;
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
