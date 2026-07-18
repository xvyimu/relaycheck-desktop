import { afterEach, describe, expect, it, vi } from "vitest";

import { checkinApi } from "../checkins";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("checkin dry-run API contract", () => {
  it("requests the server-owned all_due scope with same-origin credentials", async () => {
    const preview = {
      type: "checkin" as const,
      previewId: "preview-1",
      expiresAt: "2026-07-18T01:05:00Z",
      maxAccounts: 200 as const,
      totalAccounts: 1,
      willRun: 1,
      skipped: 0,
      items: [],
    };
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ ok: true, data: preview }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(checkinApi.previewAllDue()).resolves.toEqual(preview);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith("/api/tasks/dry-run", {
      method: "POST",
      body: JSON.stringify({ type: "checkin", scope: { kind: "all_due" } }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("preserves the API error and never attempts task start", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({ ok: false, error: "单次预览最多支持 200 个账号", errorClass: "conflict" }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(checkinApi.previewAllDue()).rejects.toThrow("单次预览最多支持 200 个账号");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/tasks/dry-run");
  });
});
