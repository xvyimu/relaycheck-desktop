import { expect, it, vi } from "vitest";

import { TaskProgressView } from "../TaskProgressView";

it("renders loading and startup-error states", () => {
  expect(TaskProgressView({ progress: null, loading: true, error: "" })).toBeTruthy();
  expect(TaskProgressView({ progress: null, loading: false, error: "连接失败", onDismiss: vi.fn() })).toBeTruthy();
});

it("renders a running batch with successful and failed items", () => {
  const element = TaskProgressView({
    progress: {
      id: "task-1",
      type: "checkin",
      status: "running",
      current: 2,
      total: 4,
      startedAt: "2026-07-17T00:00:00Z",
      updatedAt: "2026-07-17T00:01:00Z",
      results: [
        { id: "one", name: "账号一", status: "success", message: "完成" },
        { id: "two", name: "账号二", status: "failed", message: "超时" },
      ],
    },
    loading: false,
    error: "",
    onCancel: vi.fn(),
    labels: { title: "签到任务", running: "处理中", cancel: "停止" },
  });

  expect(element).toBeTruthy();
});

it("renders a completed zero-total batch with a dismiss action", () => {
  const element = TaskProgressView({
    progress: {
      id: "task-2",
      type: "test_keys",
      status: "done",
      current: 0,
      total: 0,
      results: [],
      startedAt: "2026-07-17T00:00:00Z",
      updatedAt: "2026-07-17T00:00:00Z",
    },
    loading: false,
    error: "",
    onDismiss: vi.fn(),
    labels: { done: "已结束", close: "收起" },
  });

  expect(element).toBeTruthy();
});
