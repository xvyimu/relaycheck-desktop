import { afterEach, describe, expect, it, vi } from "vitest";

import type { KeyExportPreview } from "@/types";

import { keysApi } from "../keys";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 构造脱敏 Key 导出预览；夹具不得包含真实密钥明文。 */
function makeKeyExportPreview(overrides: Partial<KeyExportPreview> = {}): KeyExportPreview {
  return {
    generatedAt: "2026-07-18T02:00:00Z",
    total: 2,
    valid: 1,
    usable: 1,
    items: [
      {
        accountId: "account-1",
        accountName: "账号一",
        siteName: "站点一",
        baseUrl: "https://relay.example",
        fingerprint: "fp-abcd",
        status: "valid",
        modelCount: 3,
        sampleModels: ["gpt-4o-mini"],
        modelUsable: true,
        maskedExportRef: "sk-****abcd",
      },
    ],
    notice: "导出仅含指纹与状态，不含真实密钥。",
    ...overrides,
  };
}

describe("keysApi", () => {
  it("只读导出预览使用固定 GET 路径，且响应原样透传", async () => {
    const preview = makeKeyExportPreview();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: preview }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(keysApi.exportPreview()).resolves.toEqual(preview);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith("/api/keys/export-preview", {
      credentials: "same-origin",
      headers: undefined,
    });

    const body = JSON.stringify(preview);
    expect(body).not.toMatch(/sk-[A-Za-z0-9]{20,}/);
    expect(body).not.toContain("Authorization");
  });
});
