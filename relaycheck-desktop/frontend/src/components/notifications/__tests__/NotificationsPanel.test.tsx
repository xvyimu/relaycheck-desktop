import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { NotificationsPanel } from "../NotificationsPanel";
import type { NotificationItem } from "@/types";

const items: NotificationItem[] = Array.from({ length: 10 }, (_, index) => ({
  id: `notification-${index}`,
  type: "test",
  level: index === 0 ? "warning" : "info",
  title: `Notification ${index}`,
  content: "Test content",
  read: index < 3,
  createdAt: "2026-07-13T00:00:00Z",
}));

describe("NotificationsPanel", () => {
  it("uses server totals instead of deriving global counts from the current page", () => {
    const html = renderToStaticMarkup(
      <NotificationsPanel
        items={items}
        total={25}
        unreadTotal={18}
        importantTotal={5}
        onRefresh={async () => undefined}
      />,
    );

    expect(html).toContain("<span>总数</span><strong>25</strong>");
    expect(html).toContain("<span>未读</span><strong>18</strong>");
    expect(html).toContain("<span>重要</span><strong>5</strong>");
    expect(html).toContain("<span>已读</span><strong>7</strong>");
  });
});
