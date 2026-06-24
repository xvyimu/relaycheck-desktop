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
      setMessage(`${label} completed.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `${label} failed.`);
    } finally {
      setBusy("");
    }
  }

  async function markAllRead() {
    await runAction("Mark all read", () => api("/api/notifications/mark-all-read", { method: "POST" }));
  }

  async function clearRead() {
    const confirmed = window.confirm(`Clear ${summary.read} read notification${summary.read === 1 ? "" : "s"}?`);
    if (!confirmed) return;
    await runAction("Clear read", () => api("/api/notifications/clear-read", { method: "POST" }));
  }

  return (
    <section className="notifications-panel">
      <div className="channel-summary notification-summary compact-summary">
        <div>
          <span>Total</span>
          <strong>{summary.total}</strong>
        </div>
        <div>
          <span>Unread</span>
          <strong>{summary.unread}</strong>
        </div>
        <div>
          <span>Important</span>
          <strong>{summary.important}</strong>
        </div>
        <div>
          <span>Read</span>
          <strong>{summary.read}</strong>
        </div>
      </div>

      <div className="notification-toolbar">
        <button
          disabled={Boolean(busy) || summary.unread === 0}
          onClick={() => void markAllRead()}
          type="button"
        >
          {busy === "Mark all read" ? "Marking..." : "Mark all read"}
        </button>
        <button
          className="ghost"
          disabled={Boolean(busy) || summary.read === 0}
          onClick={() => void clearRead()}
          type="button"
        >
          {busy === "Clear read" ? "Clearing..." : "Clear read"}
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
                  <span>{item.type || "system"}</span>
                  <strong>{item.title}</strong>
                </div>
                <span className={`badge ${tone}`}>{item.level || "info"}</span>
              </div>
              <p>{item.content}</p>
              <div className="notification-meta">
                <span>{item.read ? "Read" : "Unread"}</span>
                <span>{formatTime(item.createdAt)}</span>
              </div>
            </article>
          );
        })}

        {!items.length ? (
          <div className="empty-state">
            <div className="empty-mark">RC</div>
            <strong>No notifications</strong>
            <span>Operational events, warnings, and batch results will appear here.</span>
          </div>
        ) : null}
      </div>
    </section>
  );
}
