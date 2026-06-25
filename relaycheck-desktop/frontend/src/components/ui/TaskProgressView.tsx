import type { TaskProgress } from "@/hooks/useTaskProgress";

interface TaskProgressViewProps {
  progress: TaskProgress | null;
  loading: boolean;
  error: string;
  onCancel?: () => void;
  onDismiss?: () => void;
  labels?: {
    title?: string;
    running?: string;
    done?: string;
    cancelled?: string;
    cancel?: string;
    close?: string;
  };
}

const statusColors: Record<string, string> = {
  success: "var(--v4-success, #16a34a)",
  already_checked: "var(--v4-info, #2563eb)",
  failed: "var(--v4-danger, #dc2626)",
  unsupported: "var(--v4-neutral, #6b7280)",
  auth_expired: "var(--v4-warning, #d97706)",
  manual_required: "var(--v4-warning, #d97706)",
  valid: "var(--v4-success, #16a34a)",
  expired: "var(--v4-danger, #dc2626)",
  unknown: "var(--v4-neutral, #6b7280)",
};

const statusLabels: Record<string, string> = {
  success: "成功",
  already_checked: "今日已签",
  failed: "失败",
  unsupported: "不支持",
  auth_expired: "需授权",
  manual_required: "需手动",
  valid: "有效",
  expired: "失效",
  unknown: "未知",
};

export function TaskProgressView({
  progress,
  loading,
  error,
  onCancel,
  onDismiss,
  labels,
}: TaskProgressViewProps) {
  if (loading && !progress) {
    return (
      <div className="task-progress-card" aria-live="polite">
        <div className="task-progress-loading">正在启动任务…</div>
      </div>
    );
  }

  if (error && !progress) {
    return (
      <div className="task-progress-card" aria-live="polite">
        <div className="task-progress-error">{error}</div>
        {onDismiss ? <button type="button" className="ghost" onClick={onDismiss}>关闭</button> : null}
      </div>
    );
  }

  if (!progress) return null;

  const pct = progress.total > 0 ? Math.round((progress.current / progress.total) * 100) : 0;
  const isRunning = progress.status === "running";
  const statusText = isRunning
    ? (labels?.running || "进行中")
    : progress.status === "done"
    ? (labels?.done || "已完成")
    : (labels?.cancelled || "已取消");

  const successCount = progress.results.filter((r) => r.status === "success" || r.status === "valid" || r.status === "already_checked").length;
  const failCount = progress.results.filter((r) => r.status === "failed" || r.status === "expired").length;

  return (
    <div className="task-progress-card" aria-live="polite">
      <div className="task-progress-header">
        <span className="task-progress-title">{labels?.title || "批量任务"}</span>
        <span className={`task-progress-status ${progress.status}`}>{statusText}</span>
      </div>

      <div className="task-progress-bar-wrap">
        <div className="task-progress-bar" style={{ width: `${pct}%` }} />
      </div>

      <div className="task-progress-stats" style={{ fontVariantNumeric: "tabular-nums" }}>
        <span>{progress.current} / {progress.total}</span>
        {successCount > 0 ? <span className="task-progress-ok">成功 {successCount}</span> : null}
        {failCount > 0 ? <span className="task-progress-fail">失败 {failCount}</span> : null}
      </div>

      {progress.results.length > 0 ? (
        <div className="task-progress-results">
          {progress.results.slice(-20).map((item, i) => (
            <div key={`${item.id}-${i}`} className="task-progress-item">
              <span className="task-progress-item-name">{item.name}</span>
              <span
                className="task-progress-item-status"
                style={{ color: statusColors[item.status] || "var(--v4-text)" }}
              >
                {statusLabels[item.status] || item.status}
              </span>
              {item.message ? <span className="task-progress-item-msg">{item.message}</span> : null}
            </div>
          ))}
        </div>
      ) : null}

      <div className="task-progress-footer">
        {isRunning && onCancel ? (
          <button type="button" className="ghost" onClick={onCancel}>{labels?.cancel || "取消"}</button>
        ) : null}
        {!isRunning && onDismiss ? (
          <button type="button" className="ghost" onClick={onDismiss}>{labels?.close || "关闭"}</button>
        ) : null}
      </div>
    </div>
  );
}
