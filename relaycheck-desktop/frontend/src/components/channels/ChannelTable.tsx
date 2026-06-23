import { api } from "@/api/client";
import { CHANNELS_VISIBLE_INCREMENT } from "@/lib/constants";
import { channelInitials, formatTime } from "@/lib/format";
import {
  channelModelStatusLabel,
  channelSourceLabel,
  channelSourceSyncLabel,
  upstreamKindLabel,
} from "@/lib/labels";
import { LoadingSkeleton } from "../loading-skeleton";
import type { DetailDrawerState, ImportedChannel } from "@/types";
import type { ChannelActionsResult } from "@/hooks/useChannelActions";
import type { ChannelFiltersResult } from "@/hooks/useChannelFilters";

interface ChannelTableProps {
  channels: ImportedChannel[];
  loaded: boolean;
  message: string;
  onSetDrawer: (state: DetailDrawerState | null) => void;
  onSetMessage: (msg: string) => void;
  onRefresh: ChannelActionsResult["refresh"];
  onUpdateSourceStatus: ChannelActionsResult["updateChannelSourceStatus"];
  filters: ChannelFiltersResult;
}


export function ChannelTable({
  channels,
  loaded,
  onSetDrawer,
  onSetMessage,
  onRefresh,
  onUpdateSourceStatus,
  filters,
}: ChannelTableProps) {
  const { displayedChannels, visibleChannels, hasMoreChannels, setVisibleLimit } = filters;

  return (
    <>
      <div className="channel-grid">
        {!loaded ? (
          <LoadingSkeleton variant="table" title="正在读取渠道列表" rows={5} />
        ) : null}
        {displayedChannels.map((channel) => (
          <article
            className={`channel-card channel-card-v4 ${
              channel.baseUrl ? "" : "is-incomplete"
            } ${channel.sourceSyncStatus === "missing" ? "is-source-missing" : ""} ${
              channel.sourceSyncStatus === "archived" ? "is-source-archived" : ""
            }`}
            key={channel.id}
          >
            <div className="channel-card-head">
              <div className="channel-avatar">{channelInitials(channel.name)}</div>
              <div>
                <strong title={channel.name}>{channel.name}</strong>
                <span title={channel.baseUrl || "未配置 Base URL"}>
                  {channel.baseUrl || "未配置 Base URL"}
                </span>
              </div>
              <span className={`status-pill source-${channel.sourceSyncStatus || "active"}`}>
                <span className={`status-label level-${channel.sourceSyncStatus || "active"}`}>
                  {channelSourceSyncLabel(channel.sourceSyncStatus || "active")}
                </span>
              </span>
            </div>
            <div className="channel-card-metrics">
              <div>
                <span>后台</span>
                <strong>{upstreamKindLabel(channel.upstreamKind)}</strong>
              </div>
              <div>
                <span>签到</span>
                <strong>{channel.supportsCheckin ? "可签到" : "不可用"}</strong>
              </div>
              <div>
                <span>模型</span>
                <strong>{channel.modelCount || 0}</strong>
              </div>
            </div>
            <div className="chips secondary-chips">
              <span>来源 {channelSourceLabel(channel.sourceType || "")}</span>
              <span>签到 {channel.supportsCheckin ? "支持" : "未知/不支持"}</span>
              <span>余额 {channel.supportsBalance ? "支持" : "未知/不支持"}</span>
              {channel.supportsModels ? <span>模型列表</span> : null}
              {channel.supportsPricing ? <span>价格/倍率</span> : null}
              {channel.channelKeyMasked ? <span>Key {channel.channelKeyMasked}</span> : null}
            </div>
            {channel.sampleModels?.length ? (
              <div className="channel-model-strip">
                {channel.sampleModels.slice(0, 4).map((model) => (
                  <span key={model}>{model}</span>
                ))}
              </div>
            ) : null}
            {channel.modelsStatus ? (
              <div className="channel-subtle">
                模型同步 {channelModelStatusLabel(channel.modelsStatus)} ·{" "}
                {channel.modelsSource || "未知来源"}
                {channel.modelsLastSyncedAt
                  ? ` · ${formatTime(channel.modelsLastSyncedAt)}`
                  : ""}
              </div>
            ) : null}
            {channel.lastDetectedAt ? (
              <div className="channel-subtle">最近识别 {formatTime(channel.lastDetectedAt)}</div>
            ) : null}
            {channel.sourceSyncStatus === "missing" ? (
              <div className="problem-hint detail-hint">
                源端 channels 本次未返回该渠道
                {channel.sourceMissingAt
                  ? `，标记于 ${formatTime(channel.sourceMissingAt)}`
                  : ""}
                。本地记录已保留，未自动删除。
              </div>
            ) : null}
            {channel.sourceSyncStatus === "archived" ? (
              <div className="problem-hint detail-hint">
                该渠道已归档保留，不会参与日常关注。可以随时恢复为活跃。
              </div>
            ) : null}
            <div className="channel-actions action-dock">
              <button
                className="ghost"
                onClick={() => onSetDrawer({ kind: "channel", channel })}
              >
                详情
              </button>
              <button
                className="channel-action"
                disabled={!channel.baseUrl}
                onClick={async () => {
                  onSetMessage("");
                  await api(`/api/channels/${channel.id}/detect`, { method: "POST" });
                  onSetMessage(`${channel.name} 已识别并同步到上游站点`);
                  await onRefresh();
                }}
              >
                识别并生成站点
              </button>
              {channel.sourceSyncStatus === "missing" ? (
                <button
                  className="ghost"
                  onClick={() =>
                    void onUpdateSourceStatus(channel, "restore-source-status")
                  }
                >
                  恢复活跃
                </button>
              ) : null}
              {channel.sourceSyncStatus === "missing" ? (
                <button
                  className="danger"
                  onClick={() =>
                    void onUpdateSourceStatus(channel, "archive-source-status")
                  }
                >
                  归档保留
                </button>
              ) : null}
              {channel.sourceSyncStatus === "archived" ? (
                <button
                  className="ghost"
                  onClick={() =>
                    void onUpdateSourceStatus(channel, "restore-source-status")
                  }
                >
                  恢复活跃
                </button>
              ) : null}
            </div>
          </article>
        ))}
        {loaded && !channels.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>还没有渠道</strong>
            <span>可以先去本机扫描导入 NewAPI SQLite，或在上游站点手动添加。</span>
          </div>
        ) : null}
        {loaded && channels.length > 0 && !visibleChannels.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>没有匹配渠道</strong>
            <span>换一个关键词，或清空同步状态/后台类型筛选。</span>
          </div>
        ) : null}
      </div>
      {hasMoreChannels ? (
        <div className="load-more-row">
          <button
            type="button"
            className="ghost"
            onClick={() =>
              setVisibleLimit(
                (current: number) => current + CHANNELS_VISIBLE_INCREMENT,
              )
            }
          >
            加载更多渠道（已显示 {displayedChannels.length}/{visibleChannels.length}）
          </button>
        </div>
      ) : null}
    </>
  );
}