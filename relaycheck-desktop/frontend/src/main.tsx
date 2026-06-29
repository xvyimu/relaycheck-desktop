import { StrictMode, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { AccountsPanel } from "@/components/accounts/AccountsPanel";
import { CheckinsPanel } from "@/components/checkins/CheckinsPanel";
import { ChannelsPanel } from "@/components/channels/ChannelsPanel";
import { Dashboard } from "@/components/dashboard/Dashboard";
import { Empty } from "@/components/ui/empty";
import { Sidebar, type Tab, TABS } from "@/components/layout/Sidebar";
import { Topbar } from "@/components/layout/Topbar";
import { NotificationsPanel } from "@/components/notifications/NotificationsPanel";
import { OnboardingWizard } from "@/components/onboarding/OnboardingWizard";
import { ScanPanel } from "@/components/scan/ScanPanel";
import { Settings as SettingsPanel } from "@/components/settings/Settings";
import { SitesPanel } from "@/components/sites/SitesPanel";
import { useAppData } from "@/hooks/useAppData";
import { initTheme } from "@/lib/theme";
import type { NavigationIntent, TabKey } from "@/types";
import "./styles.css";

function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [navigationIntent, setNavigationIntent] = useState<NavigationIntent | null>(null);
  const data = useAppData();

  useEffect(() => {
    const cleanup = initTheme();
    return cleanup;
  }, []);

  if (data.loading) {
    return (
      <main className="center-screen">
        <div className="loading-card">
          正在启动 RelayCheck Desktop…
          {data.startupVersion ? <div className="loading-version">{data.startupVersion}</div> : null}
        </div>
      </main>
    );
  }

  function handleNavigate(nextTab: TabKey, intent?: Omit<NavigationIntent, "target">) {
    if (!TABS.some((item) => item.key === nextTab)) return;
    setTab(nextTab as Tab);
    setNavigationIntent({ target: nextTab, ...intent });
  }

  function handleTabChange(nextTab: Tab) {
    setTab(nextTab);
    // Tab clicks from the sidebar are plain navigations — clear any residual
    // rich intent so panels don't see stale data from a previous handleNavigate.
    setNavigationIntent(null);
  }

  return (
    <div className="app-shell">
      <OnboardingWizard />
      <Sidebar activeTab={tab} onTabChange={handleTabChange} />
      <main className="main-panel">
        <Topbar activeTab={tab} onRefresh={() => void data.reload()} />
        {data.error ? <div className="notice error" aria-live="polite">{data.error}</div> : null}
        {tab === "dashboard" ? (
          <Dashboard
            status={data.status}
            channels={data.channels}
            sites={data.sites}
            accounts={data.accounts}
            checkins={data.checkins}
            notifications={data.notifications}
            diagnostics={data.diagnostics}
            actionCenter={data.actionCenter}
            modelOverview={data.modelOverview}
            pricingOverview={data.pricingOverview}
            usageOverview={data.usageOverview}
            onNavigate={handleNavigate}
            onRefresh={data.reload}
          />
        ) : null}
        {tab === "channels" ? (
          <ChannelsPanel
            onRefresh={data.reload}
            intent={navigationIntent?.target === "channels" ? navigationIntent : null}
          />
        ) : null}
        {tab === "sites" ? (
          <SitesPanel sites={data.sites} onRefresh={data.reload} intent={navigationIntent?.target === "sites" ? navigationIntent : null} />
        ) : null}
        {tab === "accounts" ? (
          <AccountsPanel accounts={data.accounts} sites={data.sites} onRefresh={data.reload} intent={navigationIntent?.target === "accounts" ? navigationIntent : null} />
        ) : null}
        {tab === "checkins" ? (
          <CheckinsPanel checkins={data.checkins} onRefresh={data.reload} intent={navigationIntent?.target === "checkins" ? navigationIntent : null} />
        ) : null}
        {tab === "scan" ? <ScanPanel onRefresh={data.reload} /> : null}
        {tab === "notifications" ? (
          <NotificationsPanel items={data.notifications} onRefresh={data.reload} intent={navigationIntent?.target === "notifications" ? navigationIntent : null} />
        ) : null}
        {tab === "settings" ? (
          data.status ? <SettingsPanel status={data.status} onDone={data.reload} /> : <Empty message="正在加载设置…" />
        ) : null}
      </main>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
