import { useMemo, useState } from "react";

import { api } from "@/api/client";
import { formatTime } from "@/lib/format";
import type { NotificationItem } from "@/types";

type NotificationsPanelProps = {
  items: NotificationItem[];
  onRefresh: () => Promise<void>;
};

function notificationTone(level: string) {
  const normalized = level.toLowerCase();
  if (["error", "danger", "critical", "failed"].includes(normalized)) return "bad";
  if (["warning", "warn", "missing"].includes(normalized)) return "warn";
  if (["success", "ok"].includes(normalized)) return "good";
  return "neutral";
}

function isImportant(item: NotificationItem) {
  return ["error", "danger", "critical", "warning", "warn"].includes(item.level.toLowerCase());
}

export function NotificationsPanel({ items, onRefresh }: NotificationsPanelProps) {
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");

  const summary = useMemo(() => {
    const unread = items.filter((item) => !item.read).length;
    const read = items.length - unread;
    const important = items.filter(isImportant).length;
    return { total: items.length, unread, read, important };
  }, [items]);

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
          onClick={() => void clearRead()}
          type="button"
        >
          {busy === "清除已读" ? "清除中…" : "清除已读"}
        </button>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="notification-list">
        {items.map((item) => {
          const tone = notificationTone(item.level);
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

        {!items.length ? (
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
