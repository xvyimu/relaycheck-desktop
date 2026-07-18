import { afterEach, describe, expect, it, vi } from "vitest";

import { actionCenterApi } from "@/api/action-center";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("actionCenterApi", () => {
  it("loads samples for one encoded action id", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          ok: true,
          data: [{ label: "Lazy Site", entityType: "site", entityId: "site-1" }],
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      ),
    );

    await expect(actionCenterApi.samples("site issue")).resolves.toEqual([
      { label: "Lazy Site", entityType: "site", entityId: "site-1" },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/system/action-center/samples?id=site%20issue",
      expect.objectContaining({ credentials: "same-origin" }),
    );
  });
});
