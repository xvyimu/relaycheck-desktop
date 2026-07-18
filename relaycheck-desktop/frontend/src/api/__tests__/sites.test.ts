import { afterEach, describe, expect, it, vi } from "vitest";

import { sitesApi } from "../sites";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("sitesApi", () => {
  it("list 使用只读 GET /api/upstream-sites", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: [] }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );
    await expect(sitesApi.list()).resolves.toEqual([]);
    expect(fetchMock).toHaveBeenCalledWith("/api/upstream-sites", {
      credentials: "same-origin",
      headers: undefined,
    });
  });
});
