import { memo, useEffect, useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatTime } from "@/lib/format";
import { statusTone } from "@/lib/tone";
import type { NavigationIntent, NotificationItem } from "@/types";

type NotificationsPanelProps = {
  items: NotificationItem[];
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
};

function NotificationsPanelBase({ items, onRefresh, intent }: NotificationsPanelProps) {
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [showRead, setShowRead] = useState(true);

  // React to navigation intent from Action Center
  useEffect(() => {
    if (!intent) return;
    if (intent.unreadOnly) setShowRead(false);
  }, [intent]);

  const summary = useMemo(() => {
    const unread = items.filter((item) => !item.read).length;
    const read = items.length - unread;
    const important = items.filter((item) => ["error", "danger", "critical", "warning", "warn"].includes(item.level.toLowerCase())).length;
    return { total: items.length, unread, read, important };
  }, [items]);

  const visibleItems = useMemo(() => {
    return showRead ? items : items.filter((item) => !item.read);
  }, [items, showRead]);

  async function runAction(label: string, action: () => Promise<unknown>) {
    setBusy(label);
    setMessage("");
    try {
      await action();
      await onRefresh();
      setMessage(`${label}完成。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `${label}失败。`);
    } finally {
      setBusy("");
    }
  }

  async function markAllRead() {
    await runAction("全部标记已读", () => api("/api/notifications/mark-all-read", { method: "POST" }));
  }

  async function clearRead() {
    const confirmed = window.confirm(`确认清除 ${summary.read} 条已读通知？`);
    if (!confirmed) return;
    await runAction("清除已读", () => api("/api/notifications/clear-read", { method: "POST" }));
  }

  async function stowAndTrim() {
    await runAction("收纳清理", () => api("/api/notifications/trim?keep=10", { method: "POST" }));
    setShowRead(false);
  }

  return (
    <section className="notifications-panel">
      <div className="channel-summary notification-summary compact-summary">
        <div>
          <span>总数</span>
          <strong>{summary.total}</strong>
        </div>
        <div>
          <span>未读</span>
          <strong>{summary.unread}</strong>
        </div>
        <div>
          <span>重要</span>
          <strong>{summary.important}</strong>
        </div>
        <div>
          <span>已读</span>
          <strong>{summary.read}</strong>
        </div>
      </div>

      <div className="notification-toolbar">
        <button
          disabled={Boolean(busy) || summary.unread === 0}
          onClick={() => void markAllRead()}
          type="button"
        >
          {busy === "全部标记已读" ? "标记中…" : "全部标记已读"}
        </button>
        <button
          className="ghost"
          disabled={Boolean(busy) || summary.read === 0}
          onClick={() => void stowAndTrim()}
          type="button"
        >
          {busy === "收纳清理" ? "收纳中…" : `收纳已读`}
        </button>
        <button
          className="ghost"
          disabled={Boolean(busy) || summary.read === 0}
          onClick={() => void clearRead()}
          type="button"
        >
          {busy === "清除已读" ? "清除中…" : "清除已读"}
        </button>
        <button
          className="ghost"
          onClick={() => setShowRead((prev) => !prev)}
          type="button"
          style={{ marginLeft: "auto" }}
        >
          {showRead ? "仅未读" : "全部"}
        </button>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="notification-list">
        {visibleItems.map((item) => {
          const tone = statusTone(item.level, { unknown: "neutral" });
          return (
            <article
              className={`notification-card is-${item.read ? "read" : "unread"} tone-${tone}`}
              key={item.id}
            >
              <div className="notification-card-head">
                <div>
                  <span>{item.type || "系统"}</span>
                  <strong>{item.title}</strong>
                </div>
                <span className={`badge ${tone}`}>{item.level || "信息"}</span>
              </div>
              <p>{item.content}</p>
              <div className="notification-meta">
                <span>{item.read ? "已读" : "未读"}</span>
                <span>{formatTime(item.createdAt)}</span>
              </div>
            </article>
          );
        })}

        {!showRead && summary.read > 0 ? (
          <button className="ghost" onClick={() => setShowRead(true)} type="button" style={{ textAlign: "center", width: "100%", padding: "10px" }}>
            展开 {summary.read} 条已读通知
          </button>
        ) : null}

        {!visibleItems.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>暂无通知</strong>
            <span>运营事件、警告和批量结果会显示在这里。</span>
          </div>
        ) : null}
      </div>
    </section>
  );
}

export const NotificationsPanel = memo(NotificationsPanelBase);
