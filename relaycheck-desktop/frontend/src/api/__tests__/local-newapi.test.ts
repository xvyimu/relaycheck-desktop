import { afterEach, describe, expect, it, vi } from "vitest";

import { localNewapiApi, type ImportFromAdminResult } from "../local-newapi";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 构造 Admin API 导入成功夹具，字段与后端 result map 对齐。 */
function makeImportResult(overrides: Partial<ImportFromAdminResult> = {}): ImportFromAdminResult {
  return {
    instanceId: "instance-1",
    importedCount: 3,
    sitesCreated: 1,
    sitesMerged: 2,
    detectedCount: 0,
    syncTokenSaved: true,
    ...overrides,
  };
}

describe("localNewapiApi", () => {
  it("importFromAdmin 仅发送声明字段，并默认关闭 importKeys/detectAfterImport", async () => {
    const result = makeImportResult();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: result }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      localNewapiApi.importFromAdmin({
        baseUrl: " https://newapi.example ",
        accessToken: " token-1 ",
        saveAccessToken: true,
      }),
    ).resolves.toEqual(result);

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith("/api/local-newapi/import-from-admin-api", {
      method: "POST",
      body: JSON.stringify({
        baseUrl: "https://newapi.example",
        accessToken: "token-1",
        saveAccessToken: true,
        importKeys: false,
        skipCreateSites: false,
        detectAfterImport: false,
      }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("允许覆盖 importKeys/skipCreateSites/detectAfterImport，但不透传未知字段", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: makeImportResult() }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await localNewapiApi.importFromAdmin({
      baseUrl: "https://newapi.example",
      accessToken: "token-1",
      saveAccessToken: false,
      importKeys: true,
      skipCreateSites: true,
      detectAfterImport: true,
    });

    expect(fetchMock).toHaveBeenCalledWith("/api/local-newapi/import-from-admin-api", {
      method: "POST",
      body: JSON.stringify({
        baseUrl: "https://newapi.example",
        accessToken: "token-1",
        saveAccessToken: false,
        importKeys: true,
        skipCreateSites: true,
        detectAfterImport: true,
      }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });

  it("listInstances 与 excludeRules 为只读 GET", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: [] }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true, data: { rules: [], note: "n" } }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      );

    await expect(localNewapiApi.listInstances()).resolves.toEqual([]);
    await expect(localNewapiApi.excludeRules()).resolves.toEqual({ rules: [], note: "n" });
    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/local-newapi", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/local-newapi/exclude-rules", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("autoDetectImport 使用无 body 的 POST", async () => {
    const payload = {
      found: true,
      message: "ok",
      results: [{ dbPath: "a.db", baseUrl: "http://127.0.0.1", importedCount: 1, sitesCreated: 0, sitesMerged: 0 }],
    };
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true, data: payload }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(localNewapiApi.autoDetectImport()).resolves.toEqual(payload);
    expect(fetchMock).toHaveBeenCalledWith("/api/local-newapi/auto-detect-import", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("sync 默认不带令牌字段；有 draft 才附带 accessToken", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ ok: true, data: { importedCount: 1 } }), {
          status: 200,
          headers: { "content-type": "application/json" },
        }),
      ),
    );

    await localNewapiApi.sync("instance/a");
    await localNewapiApi.sync("instance-1", { accessToken: " secret " });

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/local-newapi/instance%2Fa/sync", {
      method: "POST",
      body: JSON.stringify({ importKeys: false, detectAfterImport: false, pageSize: 100 }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenNthCalledWith(2, "/api/local-newapi/instance-1/sync", {
      method: "POST",
      body: JSON.stringify({
        importKeys: false,
        detectAfterImport: false,
        pageSize: 100,
        accessToken: "secret",
        saveAccessToken: true,
      }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
  });
});
