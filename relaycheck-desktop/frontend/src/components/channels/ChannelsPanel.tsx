import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { ChannelTable } from "@/components/channels/ChannelTable";
import { DialogShell } from "@/components/ui/dialog-shell";
import { TaskProgressView } from "@/components/ui/TaskProgressView";
import { useApi } from "@/hooks/useApi";
import { useChannelActions } from "@/hooks/useChannelActions";
import { useChannelFilters } from "@/hooks/useChannelFilters";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { formatTime } from "@/lib/format";
import type { Account, ChannelHealthOverview, ChannelHealthSite, ImportedChannel, NavigationIntent } from "@/types";
import { Button } from "@/components/ui/button";

const LABELS_HEALTH_PROBE = { title: "渠道健康探测" } as const;

const emptyHealthOverview: ChannelHealthOverview = {
  generatedAt: "",
  overall: "success",
  siteCount: 0,
  healthySiteCount: 0,
  unreachableSiteCount: 0,
  channelCount: 0,
  liveModelChannelCount: 0,
  failedModelChannelCount: 0,
  uncheckedModelChannelCount: 0,
  validKeyCount: 0,
  invalidKeyCount: 0,
  uncheckedKeyCount: 0,
  sites: [],
};

export interface ChannelsPanelProps {
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
  /** When false, pause auto-fetch (keep-alive inactive tab). */
  active?: boolean;
  /** Parent bumps on tab change so keep-alive drawers release body scroll-lock. */
  dialogEpoch?: number;
  /** Inventory channels — avoids dual GET /api/channels. */
  inventoryChannels?: ImportedChannel[];
  /** Inventory accounts — avoids dual GET /api/accounts. */
  inventoryAccounts?: Account[];
}

function healthToneClass(level: string) {
  if (level === "danger") return "level-danger";
  if (level === "warning") return "level-warning";
  return "level-success";
}

function topHealthRisks(sites: ChannelHealthSite[]) {
  return sites.filter((site) => site.level === "danger" || site.level === "warning").slice(0, 4);
}

function ChannelsPanelBase({
  onRefresh,
  intent,
  active = true,
  dialogEpoch = 0,
  inventoryChannels,
  inventoryAccounts,
}: ChannelsPanelProps) {
  const actions = useChannelActions({
    active,
    initialChannels: inventoryChannels,
    initialAccounts: inventoryAccounts,
  });
  const { refresh: refreshActions, channels, accounts, setDrawer } = actions;
  useEffect(() => {
    setDrawer(null);
  }, [dialogEpoch, setDrawer]);
  const filters = useChannelFilters(channels, accounts, intent);
  const health = useApi<ChannelHealthOverview>("/api/channels/health/overview", emptyHealthOverview, {
    enabled: active,
  });
  const { refresh: refreshHealth } = health;
  const healthTask = useTaskProgress();
  const [healthProbeMessage, setHealthProbeMessage] = useState("");
  const riskSites = useMemo(() => topHealthRisks(health.data.sites), [health.data.sites]);
  const healthProgressStatus = healthTask.progress?.status;
  const healthProgressCurrent = healthTask.progress?.current;
  const healthProgressTotal = healthTask.progress?.total;

  // When not seeded from inventory, fetch once while active.
  useEffect(() => {
    if (!active) return;
    if (inventoryChannels && inventoryAccounts) return;
    void refreshActions();
  }, [active, inventoryAccounts, inventoryChannels, refreshActions]);

  const refreshAll = useCallback(async () => {
    await refreshActions();
    await refreshHealth();
    await onRefresh();
  }, [onRefresh, refreshActions, refreshHealth]);

  async function refreshHealthProbe() {
    setHealthProbeMessage("健康探测任务已启动，结果会自动刷新。");
    await healthTask.startTask("channel_health_probe", { limit: 20, onlyRisky: false });
  }

  async function syncModelsAndHealth() {
    await actions.syncChannelModels();
    await refreshHealth();
  }

  useEffect(() => {
    if (healthProgressStatus === "done") {
      setHealthProbeMessage(`健康探测完成：已处理 ${healthProgressCurrent}/${healthProgressTotal} 个站点。`);
      // actions.refresh and health.refresh catch internally, but onRefresh is
      // a parent prop whose implementation may reject; guard the call site so
      // a failure there doesn't surface as an unhandled promise rejection.
      void refreshAll().catch((err) => {
        setHealthProbeMessage(err instanceof Error ? `刷新失败：${err.message}` : "刷新失败");
      });
    } else if (healthProgressStatus === "cancelled") {
      setHealthProbeMessage("健康探测已取消。");
    }
  }, [healthProgressCurrent, healthProgressStatus, healthProgressTotal, refreshAll]);

  return (
    <section className="channels-panel">
      <section className={`channel-health-center card ${healthToneClass(health.data.overall)}`}>
        <div className="section-heading">
          <div>
            <h2>渠道健康监控</h2>
            <span>{health.loading ? "正在刷新健康概览" : `站点 ${health.data.siteCount} · 渠道 ${health.data.channelCount}`}</span>
          </div>
          <div className="toolbar">
            <Button variant="ghost" type="button" onClick={() => void refreshHealthProbe()}
              disabled={healthTask.loading || healthTask.progress?.status === "running"}
            >
              {healthTask.loading || healthTask.progress?.status === "running" ? "探测中..." : "探测健康"}
            </Button>
            <button type="button" onClick={() => void syncModelsAndHealth()} disabled={actions.modelSyncing}>
              {actions.modelSyncing ? "同步中…" : "同步模型"}
            </button>
          </div>
        </div>
        <div className="channel-health-metrics">
          <div><span>健康站点</span><strong>{health.data.healthySiteCount}</strong></div>
          <div><span>不可达</span><strong>{health.data.unreachableSiteCount}</strong></div>
          <div><span>有效 Key</span><strong>{health.data.validKeyCount}</strong></div>
          <div><span>异常 Key</span><strong>{health.data.invalidKeyCount}</strong></div>
          <div><span>实时模型</span><strong>{health.data.liveModelChannelCount}</strong></div>
          <div><span>模型异常</span><strong>{health.data.failedModelChannelCount}</strong></div>
        </div>
        {riskSites.length ? (
          <div className="channel-health-risk-list">
            {riskSites.map((site) => (
              <article className={`channel-health-risk ${healthToneClass(site.level)}`} key={site.siteId}>
                <div>
                  <span>{site.kind || "unknown"} · {site.healthStatus}</span>
                  <strong>{site.siteName}</strong>
                  <em>{site.recommendedAction}</em>
                </div>
                <div className="channel-health-risk-stats">
                  <span>异常 Key {site.invalidKeyCount}</span>
                  <span>模型异常 {site.failedModelChannelCount}</span>
                  <span>账号 {site.accountCount}</span>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="note">当前没有高优先级渠道健康风险。</div>
        )}
        {healthTask.progress || healthTask.loading || healthTask.error ? (
          <TaskProgressView
            progress={healthTask.progress}
            loading={healthTask.loading}
            error={healthTask.error}
            onCancel={healthTask.cancelTask}
            onDismiss={healthTask.reset}
            labels={LABELS_HEALTH_PROBE}
          />
        ) : null}
        {healthProbeMessage ? <div className="note">{healthProbeMessage}</div> : null}
      </section>
      <div className="channel-toolbar card">
        <div className="channel-summary compact-summary">
          <div><span>可见</span><strong>{filters.visibleChannels.length}</strong></div>
          <div><span>已识别</span><strong>{filters.identifiedCount}</strong></div>
          <div><span>目标中转</span><strong>{filters.targetRelayCount}</strong></div>
          <div><span>源端缺失</span><strong>{filters.sourceMissingCount}</strong></div>
        </div>
        <div className="proxy-form-grid">
          <label className="field">
            <span>搜索</span>
            <input value={filters.query} onChange={(event) => filters.setQuery(event.target.value)} placeholder="名称、网址、模型、账号" />
          </label>
          <label className="field">
            <span>源端状态</span>
            <select value={filters.sourceStatusFilter} onChange={(event) => filters.setSourceStatusFilter(event.target.value)}>
              <option value="not_archived">活跃 + 缺失</option>
              <option value="all">全部</option>
              <option value="active">活跃</option>
              <option value="missing">缺失</option>
              <option value="archived">已归档</option>
            </select>
          </label>
          <label className="field">
            <span>后台类型</span>
            <select value={filters.kindFilter} onChange={(event) => filters.setKindFilter(event.target.value)}>
              <option value="target_relay">目标中转</option>
              <option value="all">全部类型</option>
              {filters.kindOptions.map((kind) => (
                <option key={kind} value={kind}>{kind}</option>
              ))}
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" onClick={() => void actions.syncChannelModels()} disabled={actions.modelSyncing}>
            {actions.modelSyncing ? "同步中…" : "同步模型"}
          </button>
          <Button variant="ghost" type="button" onClick={() => void refreshAll()}>刷新</Button>
          <Button variant="ghost" type="button" onClick={filters.clearFilters}>清除筛选</Button>
        </div>
        {filters.healthFilter === "risk" ? (
          <div className="channel-active-filter">
            <div>
              <strong>健康风险筛选已启用</strong>
              <span>仅显示需要模型同步或 Key 健康复核的目标中转渠道。</span>
            </div>
            <Button variant="ghost" type="button" onClick={filters.clearFilters}>清除</Button>
          </div>
        ) : null}
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
      <DialogShell
        open={actions.drawer?.kind === "channel"}
        onClose={() => actions.setDrawer(null)}
        variant="panel"
        className="detail-drawer-wide"
        ariaLabel={
          actions.drawer?.kind === "channel" ? `渠道详情 ${actions.drawer.channel.name}` : "渠道详情"
        }
        initialFocusSelector=".detail-header .ghost, .detail-header button"
      >
        {actions.drawer?.kind === "channel" ? (
          <>
            <div className="detail-header">
              <div>
                <span className="eyebrow">渠道详情</span>
                <h2>{actions.drawer.channel.name}</h2>
              </div>
              <Button variant="ghost" type="button" onClick={() => actions.setDrawer(null)}>
                关闭
              </Button>
            </div>
            <div className="detail-grid">
              <section className="detail-card">
                <h3>运行时</h3>
                <div className="detail-list">
                  <div>
                    <span>基础网址</span>
                    <strong>{actions.drawer.channel.baseUrl || "-"}</strong>
                  </div>
                  <div>
                    <span>类型</span>
                    <strong>{actions.drawer.channel.upstreamKind || "未知"}</strong>
                  </div>
                  <div>
                    <span>模型数</span>
                    <strong>{actions.drawer.channel.modelCount || 0}</strong>
                  </div>
                  <div>
                    <span>源端</span>
                    <strong>{actions.drawer.channel.sourceSyncStatus || "活跃"}</strong>
                  </div>
                  {actions.drawer.channel.channelKeyMasked ? (
                    <div>
                      <span>API Key</span>
                      <strong className="font-mono text-xs">{actions.drawer.channel.channelKeyMasked}</strong>
                    </div>
                  ) : null}
                </div>
              </section>
              <section className="detail-card">
                <h3>能力</h3>
                <div className="chips">
                  <span>签到 {actions.drawer.channel.supportsCheckin ? "支持" : "未知/否"}</span>
                  <span>余额 {actions.drawer.channel.supportsBalance ? "支持" : "未知/否"}</span>
                  <span>模型 {actions.drawer.channel.supportsModels ? "支持" : "未知/否"}</span>
                  <span>价格 {actions.drawer.channel.supportsPricing ? "支持" : "未知/否"}</span>
                </div>
              </section>
              <section className="detail-card">
                <h3>模型</h3>
                {actions.drawer.channel.sampleModels?.length ? (
                  <div className="model-list-detail">
                    {actions.drawer.channel.sampleModels.map((model) => (
                      <span key={model} className="model-tag">
                        {model}
                      </span>
                    ))}
                  </div>
                ) : (
                  <span className="text-muted-foreground text-sm">暂无模型列表</span>
                )}
                {actions.drawer.channel.modelsStatus ? (
                  <div className="detail-list" style={{ marginTop: 8 }}>
                    <div>
                      <span>同步状态</span>
                      <strong>{actions.drawer.channel.modelsStatus}</strong>
                    </div>
                    {actions.drawer.channel.modelsSource ? (
                      <div>
                        <span>来源</span>
                        <strong>{actions.drawer.channel.modelsSource}</strong>
                      </div>
                    ) : null}
                    {actions.drawer.channel.modelsLastSyncedAt ? (
                      <div>
                        <span>最近同步</span>
                        <strong>{formatTime(actions.drawer.channel.modelsLastSyncedAt)}</strong>
                      </div>
                    ) : null}
                    {actions.drawer.channel.modelsMessage ? (
                      <div>
                        <span>消息</span>
                        <strong className="text-xs">{actions.drawer.channel.modelsMessage}</strong>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </section>
              {actions.drawer.channel.lastDetectedAt ? (
                <section className="detail-card">
                  <h3>探测</h3>
                  <div className="detail-list">
                    <div>
                      <span>最近识别</span>
                      <strong>{formatTime(actions.drawer.channel.lastDetectedAt)}</strong>
                    </div>
                  </div>
                </section>
              ) : null}
            </div>
          </>
        ) : null}
      </DialogShell>
    </section>
  );
}

export const ChannelsPanel = memo(ChannelsPanelBase);
