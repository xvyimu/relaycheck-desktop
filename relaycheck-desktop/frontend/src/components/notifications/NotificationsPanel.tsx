import { memo, useEffect, useMemo, useState } from "react";

import { notificationsApi } from "@/api/notifications";
import { formatTime } from "@/lib/format";
import { statusTone } from "@/lib/tone";
import type { NavigationIntent, NotificationItem } from "@/types";
import { Button } from "@/components/ui/button";

type NotificationsPanelProps = {
  items: NotificationItem[];
  total: number;
  unreadTotal: number;
  importantTotal: number;
  onRefresh: () => Promise<void>;
  intent?: NavigationIntent | null;
};

function NotificationsPanelBase({
  items,
  total,
  unreadTotal,
  importantTotal,
  onRefresh,
  intent,
}: NotificationsPanelProps) {
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [showRead, setShowRead] = useState(true);

  // React to navigation intent from Action Center
  useEffect(() => {
    if (!intent) return;
    if (intent.unreadOnly) setShowRead(false);
  }, [intent]);

  const summary = useMemo(() => {
    return {
      total,
      unread: unreadTotal,
      read: Math.max(0, total - unreadTotal),
      important: importantTotal,
    };
  }, [importantTotal, total, unreadTotal]);

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
    // 通知写路径归 notificationsApi，组件不拼 URL。
    await runAction("全部标记已读", () => notificationsApi.markAllRead());
  }

  async function clearRead() {
    const confirmed = window.confirm(`确认清除 ${summary.read} 条已读通知？`);
    if (!confirmed) return;
    await runAction("清除已读", () => notificationsApi.clearRead());
  }

  async function stowAndTrim() {
    await runAction("收纳清理", () => notificationsApi.trim(10));
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
        <button disabled={Boolean(busy) || summary.unread === 0} onClick={() => void markAllRead()} type="button">
          {busy === "全部标记已读" ? "标记中…" : "全部标记已读"}
        </button>
        <Button
          variant="ghost"
          disabled={Boolean(busy) || summary.read === 0}
          onClick={() => void stowAndTrim()}
          type="button"
        >
          {busy === "收纳清理" ? "收纳中…" : `收纳已读`}
        </Button>
        <Button
          variant="ghost"
          disabled={Boolean(busy) || summary.read === 0}
          onClick={() => void clearRead()}
          type="button"
        >
          {busy === "清除已读" ? "清除中…" : "清除已读"}
        </Button>
        <Button variant="ghost" onClick={() => setShowRead((prev) => !prev)} type="button" className="ml-auto">
          {showRead ? "仅未读" : "全部"}
        </Button>
      </div>

      {message ? <div className="problem-hint">{message}</div> : null}

      <div className="notification-list">
        {visibleItems.map((item) => {
          const tone = statusTone(item.level, { unknown: "neutral" });
          return (
            <article className={`notification-card is-${item.read ? "read" : "unread"} tone-${tone}`} key={item.id}>
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
          <Button variant="ghost" onClick={() => setShowRead(true)} type="button" className="w-full p-2.5 text-center">
            展开 {summary.read} 条已读通知
          </Button>
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
