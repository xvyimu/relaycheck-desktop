import { useEffect } from "react";
import { ChannelTable } from "@/components/channels/ChannelTable";
import { useChannelActions } from "@/hooks/useChannelActions";
import { useChannelFilters } from "@/hooks/useChannelFilters";

export interface ChannelsPanelProps {
  onRefresh: () => Promise<void>;
}

export function ChannelsPanel({ onRefresh }: ChannelsPanelProps) {
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
          <button type="button" className="ghost" onClick={() => void refreshAll()}>刷新</button>
          <button type="button" className="ghost" onClick={filters.clearFilters}>清除筛选</button>
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
                <span className="eyebrow">渠道详情</span>
                <h2>{actions.drawer.channel.name}</h2>
              </div>
              <button className="ghost" onClick={() => actions.setDrawer(null)}>关闭</button>
            </div>
            <div className="detail-grid">
              <section className="detail-card">
                <h3>运行时</h3>
                <div className="detail-list">
                  <div><span>基础网址</span><strong>{actions.drawer.channel.baseUrl || "-"}</strong></div>
                  <div><span>类型</span><strong>{actions.drawer.channel.upstreamKind || "未知"}</strong></div>
                  <div><span>模型</span><strong>{actions.drawer.channel.modelCount || 0}</strong></div>
                  <div><span>源端</span><strong>{actions.drawer.channel.sourceSyncStatus || "活跃"}</strong></div>
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
            </div>
          </aside>
        </div>
      ) : null}
    </section>
  );
}
