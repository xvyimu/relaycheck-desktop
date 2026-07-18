import { afterEach, describe, expect, it, vi } from "vitest";

import { accountCleanupApi, type UnsupportedCheckinCleanupPreview } from "../account-cleanup";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 创建后端清理预览夹具，统一约束 previewId 与候选账号的对应关系。 */
function makeCleanupPreview(): UnsupportedCheckinCleanupPreview {
  return {
    previewId: "cleanup-preview-1",
    expiresAt: "2026-07-18T02:05:00Z",
    matched: 1,
    deleted: 0,
    limit: 10,
    hasMore: false,
    includeLastUnsupported: true,
    items: [
      {
        accountId: "account-1",
        accountName: "待清理账号",
        upstreamSiteId: "site-1",
        upstreamSiteName: "Relay One",
        upstreamSiteKind: "oneapi",
        lastCheckinStatus: "unsupported",
        reason: "last_checkin_unsupported",
      },
    ],
  };
}

describe("accountCleanupApi", () => {
  it("用明确的 dry-run 请求生成一次性清理预览", async () => {
    const preview = makeCleanupPreview();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: preview }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(accountCleanupApi.preview({ limit: 10, includeLastUnsupported: true })).resolves.toEqual(preview);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/delete-unsupported-checkins", {
      method: "POST",
      body: JSON.stringify({ limit: 10, dryRun: true, includeLastUnsupported: true }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("确认时只提交原预览 ID，不重新提交筛选条件或候选集合", async () => {
    const result = {
      matched: 1,
      deleted: 1,
      limit: 10,
      hasMore: false,
      includeLastUnsupported: true,
      items: makeCleanupPreview().items,
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: result }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(accountCleanupApi.confirm("cleanup-preview-1")).resolves.toEqual(result);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith("/api/accounts/delete-unsupported-checkins", {
      method: "POST",
      body: JSON.stringify({ previewId: "cleanup-preview-1" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("零候选预览允许不返回 previewId 和 expiresAt", async () => {
    const empty = {
      matched: 0,
      deleted: 0,
      limit: 10,
      hasMore: false,
      includeLastUnsupported: true,
      items: [],
    };
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: empty }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(accountCleanupApi.preview({ limit: 10, includeLastUnsupported: true })).resolves.toEqual(empty);
  });
});
