import { memo, useEffect, useMemo, useState } from "react";

import { formatTime } from "@/lib/format";
import type { CheckinLog, CheckinStatus, NavigationIntent } from "@/types";
import { useTaskProgress } from "@/hooks/useTaskProgress";
import { TaskProgressView } from "@/components/ui/TaskProgressView";

const LABELS_BATCH_CHECKIN = { title: "批量签到" } as const;

type CheckinsPanelProps = {
  checkins: CheckinStatus | null;
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
};

function formatCountdown(seconds?: number) {
  if (!Number.isFinite(seconds) || !seconds || seconds <= 0) return "立即";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}天 ${hours}小时`;
  if (hours > 0) return `${hours}小时 ${minutes}分`;
  return `${Math.max(1, minutes)}分`;
}

function MetricTile({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function CheckinsPanelBase({ checkins, onRefresh, intent }: CheckinsPanelProps) {
  const [message, setMessage] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [query, setQuery] = useState("");
  const task = useTaskProgress();

  // 任务完成后刷新数据
  useEffect(() => {
    if (task.progress?.status === "done") {
      void onRefresh();
    }
  }, [task.progress?.status, onRefresh]);

  // React to navigation intent from Action Center
  useEffect(() => {
    if (!intent) return;
    if (intent.checkinStatus === "failed") setStatusFilter("failed");
    if (intent.checkinStatus === "unsupported") setStatusFilter("unsupported");
    if (intent.checkinStatus === "auth_expired") setStatusFilter("auth_expired");
    if (intent.checkinStatus === "problem") setStatusFilter("failed");
    if (typeof intent.query === "string") setQuery(intent.query);
  }, [intent]);

  const progress = useMemo(() => {
    const total = Math.max(checkins?.totalAccounts || 0, checkins?.processedAccounts || 0, 1);
    const processed = Math.min(checkins?.processedAccounts || 0, total);
    return {
      total,
      processed,
      percent: Math.round((processed / total) * 100),
    };
  }, [checkins?.processedAccounts, checkins?.totalAccounts]);

  const running = Boolean(checkins?.running);
  const today = checkins?.today;
  const schedule = checkins?.schedule;

  // Filter logs if available (based on today's logs)
  const logs = useMemo(() => {
    if (!today?.logs) return [];
    let result = [...today.logs];
    if (statusFilter === "failed") {
      result = result.filter((log: CheckinLog) => log.status === "failed");
    } else if (statusFilter === "unsupported") {
      result = result.filter((log: CheckinLog) => log.status === "unsupported");
    } else if (statusFilter === "auth_expired") {
      result = result.filter((log: CheckinLog) => log.status === "auth_expired");
    }
    if (query.trim()) {
      const normalized = query.trim().toLowerCase();
      result = result.filter((log: CheckinLog) =>
        [log.accountName || "", log.siteName || "", log.message || ""].join(" ").toLowerCase().includes(normalized)
      );
    }
    return result;
  }, [today?.logs, statusFilter, query]);

  function clearFilters() {
    setStatusFilter("all");
    setQuery("");
  }

  return (
    <section className="checkin-panel">
      <div className="channel-summary checkin-summary compact-summary">
        <div>
          <span>模式</span>
          <strong>{checkins?.mode || "待机"}</strong>
        </div>
        <div>
          <span>已处理</span>
          <strong>
            {checkins?.processedAccounts || 0}/{checkins?.totalAccounts || 0}
          </strong>
        </div>
        <div>
          <span>今日待签</span>
          <strong>{today?.dueAccounts || 0}</strong>
        </div>
        <div>
          <span>下次运行</span>
          <strong>{formatCountdown(schedule?.nextRunInSeconds)}</strong>
        </div>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="checkin-toolbar card">
        <div className="proxy-form-grid">
          <label className="field">
            <span>搜索</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="账号名、站点、消息" />
          </label>
          <label className="field">
            <span>状态</span>
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
              <option value="all">全部</option>
              <option value="failed">失败</option>
              <option value="unsupported">不支持</option>
              <option value="auth_expired">需授权</option>
            </select>
          </label>
        </div>
        <div className="toolbar">
          <button type="button" className="ghost" onClick={clearFilters}>清除筛选</button>
        </div>
        {statusFilter !== "all" ? (
          <div className="channel-active-filter">
            <div>
              <strong>签到状态筛选已启用</strong>
              <span>仅显示 {statusFilter === "failed" ? "失败" : statusFilter === "unsupported" ? "不支持" : "需授权"} 的签到记录。</span>
            </div>
            <button type="button" className="ghost" onClick={clearFilters}>清除</button>
          </div>
        ) : null}
      </div>

      <div className="checkin-grid">
        <article className="checkin-card checkin-run-card">
          <div className="section-heading">
            <div>
              <strong>运行状态</strong>
              <span>当前批次进度与活动账号。</span>
            </div>
            <span className={`status-pill ${running ? "success" : "neutral"}`}>
              {running ? "运行中" : "待机"}
            </span>
          </div>

          <div className="checkin-progress" aria-label="签到进度">
            <div>
              <span>{progress.percent}%</span>
              <strong>
                {progress.processed}/{progress.total}
              </strong>
            </div>
            <div
              aria-valuemax={progress.total}
              aria-valuemin={0}
              aria-valuenow={progress.processed}
              className="checkin-progress-track"
              role="progressbar"
            >
              <span style={{ width: `${progress.percent}%` }} />
            </div>
          </div>

          <dl className="kv checkin-kv">
            <dt>当前账号</dt>
            <dd>{checkins?.currentAccount || "-"}</dd>
            <dt>当前站点</dt>
            <dd>{checkins?.currentSite || "-"}</dd>
            <dt>待处理</dt>
            <dd>{checkins?.pendingAccounts ?? 0}</dd>
            <dt>开始时间</dt>
            <dd>{formatTime(checkins?.startedAt || "")}</dd>
            <dt>更新时间</dt>
            <dd>{formatTime(checkins?.updatedAt || "")}</dd>
          </dl>

          {checkins?.currentMessage || checkins?.lastRunMessage ? (
            <div className="problem-hint detail-hint">
              {checkins.currentMessage || checkins.lastRunMessage}
            </div>
          ) : null}

          <button
            className="wide"
            disabled={task.loading || running || task.progress?.status === "running"}
            onClick={() => void task.startTask("checkin")}
            type="button"
          >
            {task.loading || task.progress?.status === "running" ? "运行中…" : "执行全部签到"}
          </button>

          <TaskProgressView
            progress={task.progress}
            loading={task.loading}
            error={task.error}
            onCancel={task.cancelTask}
            onDismiss={task.reset}
            labels={LABELS_BATCH_CHECKIN}
          />
        </article>

        <article className="checkin-card">
          <div className="section-heading">
            <div>
              <strong>今日</strong>
              <span>今日签到结果分布。</span>
            </div>
          </div>
          <div className="checkin-metrics">
            <MetricTile label="成功" value={today?.successCount || 0} />
            <MetricTile label="已签" value={today?.alreadyCount || 0} />
            <MetricTile label="失败" value={today?.failedCount || 0} />
            <MetricTile label="不支持" value={today?.unsupportedCount || 0} />
            <MetricTile label="需授权" value={today?.authExpiredCount || 0} />
            <MetricTile label="日志" value={today?.totalLogs || 0} />
          </div>

          {logs.length > 0 && (
            <div className="checkin-logs">
              <div className="log-list">
                {logs.map((log: CheckinLog) => (
                  <div key={log.id} className={`log-item ${log.status}`}>
                    <div className="log-main">
                      <span className="log-account">{log.accountName || "未知账号"}</span>
                      <span className="log-site">{log.siteName || "未知站点"}</span>
                      <span className={`log-status status-pill ${log.status === "failed" ? "danger" : log.status === "unsupported" || log.status === "auth_expired" ? "warning" : "success"}`}>
                        {log.status === "failed" ? "失败" : log.status === "unsupported" ? "不支持" : log.status === "auth_expired" ? "需授权" : log.status === "success" ? "成功" : log.status}
                      </span>
                    </div>
                    {log.message && <div className="log-message">{log.message}</div>}
                    <div className="log-time">{formatTime(log.createdAt || "")}</div>
                  </div>
                ))}
              </div>
            </div>
          )}
          {!logs.length && today?.totalLogs ? (
            <div className="empty-state">
              <div className="empty-mark">RC</div>
              <strong>暂无匹配的签到记录</strong>
              <span>当前筛选条件下没有找到记录。尝试清除筛选。</span>
            </div>
          ) : null}
        </article>

        <article className="checkin-card">
          <div className="section-heading">
            <div>
              <strong>计划</strong>
              <span>自动化窗口与下次执行时间。</span>
            </div>
            <span className={`status-pill ${schedule?.enabled ? "success" : "neutral"}`}>
              {schedule?.enabled ? "已启用" : "未启用"}
            </span>
          </div>
          <dl className="kv checkin-kv">
            <dt>时间</dt>
            <dd>{schedule?.time || "-"}</dd>
            <dt>随机延迟</dt>
            <dd>
              {schedule ? `${schedule.randomDelayMin}-${schedule.randomDelayMax} 分钟` : "-"}
            </dd>
            <dt>窗口开始</dt>
            <dd>{formatTime(schedule?.nextWindowStartAt || "")}</dd>
            <dt>窗口结束</dt>
            <dd>{formatTime(schedule?.nextWindowEndAt || "")}</dd>
            <dt>下次运行</dt>
            <dd>{formatTime(schedule?.nextRunAt || "")}</dd>
            <dt>倒计时</dt>
            <dd>{formatCountdown(schedule?.nextRunInSeconds)}</dd>
          </dl>
          {schedule?.message ? <div className="note">{schedule.message}</div> : null}
      </article>
      </div>
    </section>
  );
}

export const CheckinsPanel = memo(CheckinsPanelBase);
