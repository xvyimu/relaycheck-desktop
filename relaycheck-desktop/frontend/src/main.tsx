import { StrictMode, Suspense, lazy, useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Dashboard } from "@/components/dashboard/Dashboard";
import { Empty } from "@/components/ui/empty";
import { LoadingSkeleton } from "@/components/loading-skeleton";
import { Sidebar, type Tab, TABS } from "@/components/layout/Sidebar";
import { Topbar } from "@/components/layout/Topbar";
import { useInventoryData } from "@/hooks/useInventoryData";
import { useModelUsageOverview } from "@/hooks/useModelUsageOverview";
import { useOpsHealth } from "@/hooks/useOpsHealth";
import { useSystemOverview } from "@/hooks/useSystemOverview";
import { appIsInitialLoading, refreshAppData } from "@/lib/appData";
import { hasEvictableTabs, IDLE_TAB_TTL_MS, pruneIdleTabs } from "@/lib/idle-tabs";
import { initTheme } from "@/lib/theme";
import type { NavigationIntent, TabKey } from "@/types";
import "./styles.css";

const PINNED_TABS = new Set<Tab>(["dashboard"]);
const PRUNE_INTERVAL_MS = 30_000;

const ChannelsPanel = lazy(() =>
  import("@/components/channels/ChannelsPanel").then((m) => ({ default: m.ChannelsPanel })),
);
const SitesPanel = lazy(() => import("@/components/sites/SitesPanel").then((m) => ({ default: m.SitesPanel })));
const CheckinsPanel = lazy(() =>
  import("@/components/checkins/CheckinsPanel").then((m) => ({ default: m.CheckinsPanel })),
);
const ScanPanel = lazy(() => import("@/components/scan/ScanPanel").then((m) => ({ default: m.ScanPanel })));
const NotificationsPanel = lazy(() =>
  import("@/components/notifications/NotificationsPanel").then((m) => ({ default: m.NotificationsPanel })),
);
const SettingsPanel = lazy(() => import("@/components/settings/Settings").then((m) => ({ default: m.Settings })));
const OnboardingWizard = lazy(() =>
  import("@/components/onboarding/OnboardingWizard").then((m) => ({ default: m.OnboardingWizard })),
);

function PanelFallback() {
  return <LoadingSkeleton variant="panel" title="正在加载面板…" rows={3} />;
}

function App() {
  const [tab, setTab] = useState<Tab>("dashboard");
  const [navigationIntent, setNavigationIntent] = useState<NavigationIntent | null>(null);
  // Keep visited panels mounted (filters/scroll). Non-pinned idle tabs drop after TTL.
  const [visitedTabs, setVisitedTabs] = useState<Set<Tab>>(() => new Set(["dashboard"]));
  const lastVisitedAtRef = useRef<Map<Tab, number>>(new Map([["dashboard", Date.now()]]));
  const tabRef = useRef<Tab>(tab);
  tabRef.current = tab;

  const system = useSystemOverview();
  const inventory = useInventoryData();
  const ops = useOpsHealth();
  const modelUsage = useModelUsageOverview();

  useEffect(() => {
    const cleanup = initTheme();
    return cleanup;
  }, []);

  const markVisited = useCallback((target: Tab) => {
    lastVisitedAtRef.current.set(target, Date.now());
    setVisitedTabs((prev) => {
      if (prev.has(target)) return prev;
      const next = new Set(prev);
      next.add(target);
      return next;
    });
  }, []);

  useEffect(() => {
    if (!hasEvictableTabs(visitedTabs, PINNED_TABS)) return;
    const timer = window.setInterval(() => {
      const now = Date.now();
      const active = tabRef.current;
      setVisitedTabs((prev) => {
        const pruned = pruneIdleTabs(prev, active, lastVisitedAtRef.current, now, {
          ttlMs: IDLE_TAB_TTL_MS,
          pinned: PINNED_TABS,
        });
        if (!pruned) return prev;
        for (const key of prev) {
          if (!pruned.has(key)) lastVisitedAtRef.current.delete(key);
        }
        return pruned;
      });
    }, PRUNE_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [visitedTabs]);

  const handleNavigate = useCallback(
    (nextTab: TabKey, intent?: Omit<NavigationIntent, "target">) => {
      if (!TABS.some((item) => item.key === nextTab)) return;
      const target = nextTab as Tab;
      setTab(target);
      setNavigationIntent({ target, ...intent });
      markVisited(target);
    },
    [markVisited],
  );

  const handleTabChange = useCallback(
    (nextTab: Tab) => {
      setTab(nextTab);
      // Sidebar clicks clear residual rich intent from handleNavigate.
      setNavigationIntent(null);
      markVisited(nextTab);
    },
    [markVisited],
  );

  const reload = useCallback(async () => {
    await refreshAppData(
      { refresh: system.refresh },
      { refresh: inventory.refresh },
      { refresh: ops.refresh },
      { refresh: modelUsage.refresh },
    );
  }, [inventory.refresh, modelUsage.refresh, ops.refresh, system.refresh]);

  const handleRefresh = useCallback(() => {
    void reload();
  }, [reload]);

  const loading = appIsInitialLoading(system, inventory, ops);
  const error = system.error || inventory.error || ops.error || modelUsage.error;

  if (loading) {
    return (
      <main className="center-screen">
        <div className="loading-card">
          正在启动 RelayCheck Desktop…
          {system.startupVersion ? <div className="loading-version">{system.startupVersion}</div> : null}
        </div>
      </main>
    );
  }

  const show = (key: Tab) => (tab === key ? undefined : "none");

  return (
    <div className="app-shell">
      <Suspense fallback={null}>
        <OnboardingWizard />
      </Suspense>
      <Sidebar activeTab={tab} onTabChange={handleTabChange} />
      <main className="main-panel">
        <Topbar activeTab={tab} onRefresh={handleRefresh} />
        {error ? (
          <div className="notice error" aria-live="polite">
            {error}
          </div>
        ) : null}
        {visitedTabs.has("dashboard") ? (
          <div style={{ display: show("dashboard") }} aria-hidden={tab !== "dashboard"} inert={tab !== "dashboard"}>
            <Dashboard
              system={system}
              inventory={inventory}
              ops={ops}
              modelUsage={modelUsage}
              onNavigate={handleNavigate}
              onRefresh={reload}
            />
          </div>
        ) : null}
        {visitedTabs.has("channels") ? (
          <div style={{ display: show("channels") }} aria-hidden={tab !== "channels"} inert={tab !== "channels"}>
            <Suspense fallback={<PanelFallback />}>
              <ChannelsPanel
                onRefresh={reload}
                intent={navigationIntent?.target === "channels" ? navigationIntent : null}
                active={tab === "channels"}
                inventoryChannels={inventory.channels}
                inventoryAccounts={inventory.accounts}
              />
            </Suspense>
          </div>
        ) : null}
        {visitedTabs.has("sites") ? (
          <div style={{ display: show("sites") }} aria-hidden={tab !== "sites"} inert={tab !== "sites"}>
            <Suspense fallback={<PanelFallback />}>
              <SitesPanel
                sites={inventory.sites}
                accounts={inventory.accounts}
                onRefresh={reload}
                intent={navigationIntent?.target === "sites" ? navigationIntent : null}
                onNavigate={handleNavigate}
              />
            </Suspense>
          </div>
        ) : null}
        {visitedTabs.has("checkins") ? (
          <div style={{ display: show("checkins") }} aria-hidden={tab !== "checkins"} inert={tab !== "checkins"}>
            <Suspense fallback={<PanelFallback />}>
              <CheckinsPanel
                checkins={ops.checkins}
                onRefresh={reload}
                intent={navigationIntent?.target === "checkins" ? navigationIntent : null}
              />
            </Suspense>
          </div>
        ) : null}
        {visitedTabs.has("scan") ? (
          <div style={{ display: show("scan") }} aria-hidden={tab !== "scan"} inert={tab !== "scan"}>
            <Suspense fallback={<PanelFallback />}>
              <ScanPanel onRefresh={reload} />
            </Suspense>
          </div>
        ) : null}
        {visitedTabs.has("notifications") ? (
          <div style={{ display: show("notifications") }} aria-hidden={tab !== "notifications"} inert={tab !== "notifications"}>
            <Suspense fallback={<PanelFallback />}>
              <NotificationsPanel
                items={ops.notifications}
                onRefresh={reload}
                intent={navigationIntent?.target === "notifications" ? navigationIntent : null}
              />
            </Suspense>
          </div>
        ) : null}
        {visitedTabs.has("settings") ? (
          <div style={{ display: show("settings") }} aria-hidden={tab !== "settings"} inert={tab !== "settings"}>
            <Suspense fallback={<PanelFallback />}>
              {system.status ? <SettingsPanel status={system.status} onDone={reload} /> : <Empty message="正在加载设置…" />}
            </Suspense>
          </div>
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
