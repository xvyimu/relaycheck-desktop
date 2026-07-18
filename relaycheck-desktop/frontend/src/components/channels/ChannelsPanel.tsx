import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { channelsApi } from "@/api/channels";
import { ChannelTable } from "@/components/channels/ChannelTable";
import { DialogShell } from "@/components/ui/dialog-shell";
import { TaskProgressView } from "@/components/ui/TaskProgressView";
import { useApi } from "@/hooks/useApi";
import { useChannelActions } from "@/hooks/useChannelActions";
import { useChannelFilters } from "@/hooks/useChannelFilters";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { formatTime } from "@/lib/format";
import type { ChannelHealthOverview, ChannelHealthSite, ImportedChannel, NavigationIntent } from "@/types";
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
}

/**
 * 并行刷新模型、健康与 inventory；任一失败不阻断其它路径，
 * 返回部分失败文案供面板展示，成功时返回空串。
 */
export async function refreshChannelPanelData(
  refreshModels: () => Promise<void>,
  refreshHealth: () => Promise<void>,
  refreshInventory: () => Promise<void>,
): Promise<string> {
  const results = await Promise.allSettled([refreshModels(), refreshHealth(), refreshInventory()]);
  const failed = results.filter((result) => result.status === "rejected").length;
  if (failed === 0) return "";
  return `部分刷新失败（${failed}/3），已保留成功更新的数据。`;
}

/**
 * 先同步渠道模型，再刷新健康概览。
 * 保证工具栏“同步模型”与健康卡片同按钮在同一所有权链路上顺序执行。
 */
export async function syncChannelModelsAndHealth(
  syncModels: () => Promise<void>,
  refreshHealth: () => Promise<void>,
): Promise<void> {
  await syncModels();
  await refreshHealth();
}

/**
 * inventory 已注入时不再由面板发起模型 autoload，避免与 Dashboard inventory 双请求。
 */
export function shouldAutoloadChannelModels(active: boolean, inventoryChannels?: ImportedChannel[]): boolean {
  return active && inventoryChannels === undefined;
}

/** 将健康等级映射为面板 tone class，未知值安全回落到 success。 */
export function healthToneClass(level: string) {
  if (level === "danger") return "level-danger";
  if (level === "warning") return "level-warning";
  return "level-success";
}

/** 取最多 4 个 danger/warning 站点作为健康风险卡片数据源。 */
export function topHealthRisks(sites: ChannelHealthSite[]) {
  return sites.filter((site) => site.level === "danger" || site.level === "warning").slice(0, 4);
}

function ChannelsPanelBase({
  onRefresh,
  intent,
  active = true,
  dialogEpoch = 0,
  inventoryChannels,
}: ChannelsPanelProps) {
  const actions = useChannelActions({
    active,
    initialChannels: inventoryChannels,
    onInventoryRefresh: onRefresh,
  });
  const { refresh: refreshActions, channels, setDrawer, setMessage } = actions;
  useEffect(() => {
    setDrawer(null);
  }, [dialogEpoch, setDrawer]);
  const filters = useChannelFilters(channels, intent, active);
  // 健康概览路径归 channelsApi 所有，面板禁止手写 /api/channels/*。
  const health = useApi<ChannelHealthOverview>(channelsApi.healthOverviewPath, emptyHealthOverview, {
    enabled: active,
  });
  const { refresh: refreshHealth } = health;
  const healthTask = useTaskProgress();
  const [healthProbeMessage, setHealthProbeMessage] = useState("");
  const riskSites = useMemo(() => topHealthRisks(health.data.sites), [health.data.sites]);
  const healthProgressStatus = healthTask.progress?.status;
  const healthProgressCurrent = healthTask.progress?.current;
  const healthProgressTotal = healthTask.progress?.total;

  // inventory 已注入时不 autoload 模型，避免 dual GET。
  useEffect(() => {
    if (!shouldAutoloadChannelModels(active, inventoryChannels)) return;
    void refreshActions();
  }, [active, inventoryChannels, refreshActions]);

  const refreshAll = useCallback(async () => {
    const partialError = await refreshChannelPanelData(refreshActions, refreshHealth, onRefresh);
    if (partialError) setMessage(partialError);
  }, [onRefresh, refreshActions, refreshHealth, setMessage]);

  async function refreshHealthProbe() {
    setHealthProbeMessage("健康探测任务已启动，结果会自动刷新。");
    await healthTask.startTask("channel_health_probe", { limit: 20, onlyRisky: false });
  }

  async function syncModelsAndHealth() {
    await syncChannelModelsAndHealth(actions.syncChannelModels, refreshHealth);
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
            <span>
              {health.loading ? "正在刷新健康概览" : `站点 ${health.data.siteCount} · 渠道 ${health.data.channelCount}`}
            </span>
          </div>
          <div className="toolbar">
            <Button
              variant="ghost"
              type="button"
              onClick={() => void refreshHealthProbe()}
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
          <div>
            <span>健康站点</span>
            <strong>{health.data.healthySiteCount}</strong>
          </div>
          <div>
            <span>不可达</span>
            <strong>{health.data.unreachableSiteCount}</strong>
          </div>
          <div>
            <span>有效 Key</span>
            <strong>{health.data.validKeyCount}</strong>
          </div>
          <div>
            <span>异常 Key</span>
            <strong>{health.data.invalidKeyCount}</strong>
          </div>
          <div>
            <span>实时模型</span>
            <strong>{health.data.liveModelChannelCount}</strong>
          </div>
          <div>
            <span>模型异常</span>
            <strong>{health.data.failedModelChannelCount}</strong>
          </div>
        </div>
        {riskSites.length ? (
          <div className="channel-health-risk-list">
            {riskSites.map((site) => (
              <article className={`channel-health-risk ${healthToneClass(site.level)}`} key={site.siteId}>
                <div>
                  <span>
                    {site.kind || "unknown"} · {site.healthStatus}
                  </span>
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
          <div>
            <span>可见</span>
            <strong>{filters.visibleChannels.length}</strong>
          </div>
          <div>
            <span>已识别</span>
            <strong>{filters.identifiedCount}</strong>
          </div>
          <div>
            <span>目标中转</span>
            <strong>{filters.targetRelayCount}</strong>
          </div>
          <div>
            <span>源端缺失</span>
            <strong>{filters.sourceMissingCount}</strong>
          </div>
        </div>
        <div className="proxy-form-grid">
          <label className="field">
            <span>搜索</span>
            <input
              value={filters.query}
              onChange={(event) => filters.setQuery(event.target.value)}
              onCompositionStart={() => filters.setQueryComposing(true)}
              onCompositionEnd={(event) => {
                filters.setQuery(event.currentTarget.value);
                filters.setQueryComposing(false);
              }}
              placeholder="名称、网址、模型、账号"
            />
          </label>
          <label className="field">
            <span>源端状态</span>
            <select
              value={filters.sourceStatusFilter}
              onChange={(event) => filters.setSourceStatusFilter(event.target.value)}
            >
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
                <option key={kind} value={kind}>
                  {kind}
                </option>
              ))}
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" onClick={() => void actions.syncChannelModels()} disabled={actions.modelSyncing}>
            {actions.modelSyncing ? "同步中…" : "同步模型"}
          </button>
          <Button variant="ghost" type="button" onClick={() => void refreshAll()}>
            刷新
          </Button>
          <Button variant="ghost" type="button" onClick={filters.clearFilters}>
            清除筛选
          </Button>
        </div>
        {filters.healthFilter === "risk" ? (
          <div className="channel-active-filter">
            <div>
              <strong>健康风险筛选已启用</strong>
              <span>仅显示需要模型同步或 Key 健康复核的目标中转渠道。</span>
            </div>
            <Button variant="ghost" type="button" onClick={filters.clearFilters}>
              清除
            </Button>
          </div>
        ) : null}
        {filters.accountSearchLoading ? <div className="note">正在搜索账号关联站点…</div> : null}
        {filters.accountSearchTruncated ? (
          <div className="note">账号匹配站点较多，仅显示前 200 个匹配站点。</div>
        ) : null}
        {filters.accountSearchError ? <div className="note">账号搜索失败：{filters.accountSearchError}</div> : null}
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
        ariaLabel={actions.drawer?.kind === "channel" ? `渠道详情 ${actions.drawer.channel.name}` : "渠道详情"}
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
                  <div className="detail-list spacing-top-sm">
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
