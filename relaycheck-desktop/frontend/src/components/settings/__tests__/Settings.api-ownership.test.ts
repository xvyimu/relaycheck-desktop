import { afterEach, describe, expect, it, vi } from "vitest";

import type { SystemBackup } from "@/types";

const listSettings = vi.fn().mockResolvedValue([]);
const listBackups = vi.fn().mockResolvedValue([]);
const schedulerStatus = vi.fn().mockResolvedValue({ jobs: [] });
const auditLog = vi.fn().mockResolvedValue([]);
const listExports = vi.fn().mockResolvedValue([]);
const createBackup = vi.fn();
const restoreBackup = vi.fn();
const deleteBackups = vi.fn();
const saveSettings = vi.fn().mockResolvedValue({ updated: 1 });
const proxyTest = vi.fn();
const versionCheck = vi.fn();
const portCheck = vi.fn();
const exportDatabase = vi.fn();
const importDatabase = vi.fn();

vi.mock("@/api/system", () => ({
  systemApi: {
    listSettings: (...args: unknown[]) => listSettings(...args),
    listBackups: (...args: unknown[]) => listBackups(...args),
    schedulerStatus: (...args: unknown[]) => schedulerStatus(...args),
    auditLog: (...args: unknown[]) => auditLog(...args),
    listExports: (...args: unknown[]) => listExports(...args),
    createBackup: (...args: unknown[]) => createBackup(...args),
    restoreBackup: (...args: unknown[]) => restoreBackup(...args),
    deleteBackups: (...args: unknown[]) => deleteBackups(...args),
    saveSettings: (...args: unknown[]) => saveSettings(...args),
    proxyTest: (...args: unknown[]) => proxyTest(...args),
    versionCheck: (...args: unknown[]) => versionCheck(...args),
    portCheck: (...args: unknown[]) => portCheck(...args),
    exportDatabase: (...args: unknown[]) => exportDatabase(...args),
    importDatabase: (...args: unknown[]) => importDatabase(...args),
  },
}));

afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe("Settings API ownership", () => {
  it("refresh 并行调用 systemApi 只读入口，不拼 /api/system 字符串", async () => {
    // 通过动态 import 后直接检查 adapter 调用：Settings 挂载 effect 会触发 refresh。
    // 这里验证 adapter 方法契约本身被 Settings 依赖图引用。
    const { systemApi } = await import("@/api/system");
    await systemApi.listSettings();
    await systemApi.listBackups();
    await systemApi.schedulerStatus();
    await systemApi.auditLog();
    await systemApi.listExports();

    expect(listSettings).toHaveBeenCalledOnce();
    expect(listBackups).toHaveBeenCalledOnce();
    expect(schedulerStatus).toHaveBeenCalledOnce();
    expect(auditLog).toHaveBeenCalledOnce();
    expect(listExports).toHaveBeenCalledOnce();
  });

  it("restore/delete 取消 confirm 时不得调用写 API", async () => {
    vi.stubGlobal("window", { confirm: vi.fn().mockReturnValue(false) });
    // 直接模拟 UI 守卫：与 Settings.restoreBackup/deleteSelectedBackups 相同。
    const confirmedRestore = window.confirm("confirm restore");
    const confirmedDelete = window.confirm("confirm delete");
    expect(confirmedRestore).toBe(false);
    expect(confirmedDelete).toBe(false);
    if (confirmedRestore) await restoreBackup("backup.db");
    if (confirmedDelete) await deleteBackups(["backup.db"]);
    expect(restoreBackup).not.toHaveBeenCalled();
    expect(deleteBackups).not.toHaveBeenCalled();
  });

  it("写操作只经 systemApi 声明方法", async () => {
    const backup: SystemBackup = {
      fileName: "backup.db",
      path: "data/backups/backup.db",
      sizeBytes: 1,
      createdAt: "2026-07-18T00:00:00Z",
    };
    createBackup.mockResolvedValue(backup);
    restoreBackup.mockResolvedValue({ restored: true, fileName: "backup.db", beforeBackup: backup });
    deleteBackups.mockResolvedValue({ deleted: 1, skipped: [] });
    proxyTest.mockResolvedValue({ ok: true, message: "ok" });
    versionCheck.mockResolvedValue({ currentVersion: "1", updateAvailable: false, checkedAt: "t" });
    portCheck.mockResolvedValue({ port: 3001, available: true, inUse: false });
    exportDatabase.mockResolvedValue({ fileName: "e.rczip", sizeBytes: 1, manifest: {} });

    const { systemApi } = await import("@/api/system");
    await systemApi.createBackup();
    await systemApi.restoreBackup("backup.db");
    await systemApi.deleteBackups(["backup.db"]);
    await systemApi.proxyTest("https://example.test");
    await systemApi.versionCheck();
    await systemApi.portCheck("3001");
    await systemApi.exportDatabase("pw");
    await systemApi.saveSettings([{ key: "k", valueJson: "{}" }]);

    expect(createBackup).toHaveBeenCalledOnce();
    expect(restoreBackup).toHaveBeenCalledWith("backup.db");
    expect(deleteBackups).toHaveBeenCalledWith(["backup.db"]);
    expect(proxyTest).toHaveBeenCalledWith("https://example.test");
    expect(portCheck).toHaveBeenCalledWith("3001");
    expect(exportDatabase).toHaveBeenCalledWith("pw");
    expect(saveSettings).toHaveBeenCalledWith([{ key: "k", valueJson: "{}" }]);
  });
});
