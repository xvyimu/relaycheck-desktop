import { afterEach, describe, expect, it, vi } from "vitest";

import { systemApi } from "../system";

afterEach(() => {
  vi.restoreAllMocks();
});

/** 统一 mock 成功 JSON，避免 POST 复用 Response 体流。 */
function mockOk(data: unknown = {}) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify({ ok: true, data }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    ),
  );
}

describe("systemApi", () => {
  it("只读加载路径使用固定 GET", async () => {
    const fetchMock = mockOk([]);
    await systemApi.listSettings();
    await systemApi.listBackups();
    await systemApi.schedulerStatus();
    await systemApi.auditLog();
    await systemApi.listExports();
    await systemApi.versionCheck();

    expect(fetchMock).toHaveBeenCalledWith("/api/system/settings", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/backups", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/scheduler-status", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/audit-log", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/exports", {
      credentials: "same-origin",
      headers: undefined,
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/version-check", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("portCheck 对端口 encode 且不发 body", async () => {
    const fetchMock = mockOk({ port: 3001, available: true, inUse: false });
    await systemApi.portCheck("3001");
    expect(fetchMock).toHaveBeenCalledWith("/api/system/port-check?port=3001", {
      credentials: "same-origin",
      headers: undefined,
    });
  });

  it("写路径仅发送声明字段", async () => {
    const fetchMock = mockOk({ updated: 1 });
    await systemApi.saveSettings([{ key: "network.proxy", valueJson: "{}" }]);
    await systemApi.restoreBackup("backup.db");
    await systemApi.deleteBackups(["a.db", "b.db"]);
    await systemApi.proxyTest("https://example.test");
    await systemApi.exportDatabase("pw");
    await systemApi.importDatabase("pw", "export.rczip");
    await systemApi.createBackup();

    expect(fetchMock).toHaveBeenCalledWith("/api/system/settings", {
      method: "PUT",
      body: JSON.stringify({ settings: [{ key: "network.proxy", valueJson: "{}" }] }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/restore", {
      method: "POST",
      body: JSON.stringify({ fileName: "backup.db" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/backups/delete", {
      method: "POST",
      body: JSON.stringify({ fileNames: ["a.db", "b.db"] }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/proxy-test", {
      method: "POST",
      body: JSON.stringify({ targetUrl: "https://example.test" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/export", {
      method: "POST",
      body: JSON.stringify({ password: "pw" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/import", {
      method: "POST",
      body: JSON.stringify({ password: "pw", fileName: "export.rczip" }),
      credentials: "same-origin",
      headers: { "content-type": "application/json" },
    });
    expect(fetchMock).toHaveBeenCalledWith("/api/system/backup", {
      method: "POST",
      credentials: "same-origin",
      headers: undefined,
    });
  });
});
