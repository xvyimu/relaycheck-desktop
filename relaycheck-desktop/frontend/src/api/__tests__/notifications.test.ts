import { afterEach, describe, expect, it, vi } from "vitest";

import { notificationsApi } from "../notifications";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("notificationsApi", () => {
  it("markAllRead 与 clearRead 使用固定 POST 路径", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: {} }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    await notificationsApi.markAllRead();
    await notificationsApi.clearRead();

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/notifications/mark-all-read", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/notifications/clear-read", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("trim 默认 keep=10，并允许覆盖", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: {} }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    await notificationsApi.trim();
    await notificationsApi.trim(20);

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/notifications/trim?keep=10", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/notifications/trim?keep=20", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });
});
