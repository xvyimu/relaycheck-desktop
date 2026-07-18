import { describe, expect, it } from "vitest";
import { useChannelActions } from "@/hooks/useChannelActions";
import { SettingsProxy } from "@/components/settings/SettingsProxy";
import { SettingsBackup } from "@/components/settings/SettingsBackup";
import { SettingsExportImport } from "@/components/settings/SettingsExportImport";

describe("S2 FE-7 panel contracts", () => {
  it("useChannelActions is a function accepting optional seed options", () => {
    expect(typeof useChannelActions).toBe("function");
    expect(useChannelActions.length).toBeLessThanOrEqual(1);
  });

  it("settings split modules export function components", () => {
    expect(typeof SettingsProxy).toBe("function");
    expect(typeof SettingsBackup).toBe("function");
    expect(typeof SettingsExportImport).toBe("function");
  });

  it("renders populated proxy, backup, and encrypted export states", () => {
    const noop = () => undefined;
    const proxy = SettingsProxy({
      proxyConfig: { enabled: true, url: "http://127.0.0.1:7897", bypassLocal: true },
      proxyTestTarget: "https://example.test",
      proxyTestResult: {
        ok: true,
        targetUrl: "https://example.test",
        httpStatus: 200,
        latencyMs: 12,
        message: "ok",
        proxy: {
          enabled: true,
          url: "http://127.0.0.1:7897",
          urlMasked: "http://127.0.0.1:7897",
          bypassLocal: true,
        },
      },
      busy: false,
      canSave: true,
      defaultConfig: { enabled: false, url: "", bypassLocal: true },
      onPatch: noop,
      onTargetChange: noop,
      onTest: noop,
      onReset: noop,
    });
    const backup = SettingsBackup({
      backups: [
        { fileName: "backup.db", path: "data/backups/backup.db", sizeBytes: 1024, createdAt: "2026-07-17T00:00:00Z" },
      ],
      busy: false,
      multiSelect: true,
      selected: ["backup.db"],
      onRefresh: noop,
      onToggleMulti: noop,
      onToggleSelect: noop,
      onDeleteSelected: noop,
      onRestore: noop,
    });
    const exportImport = SettingsExportImport({
      exportPassword: "secret1",
      importPassword: "secret2",
      importFileName: "export.rczip",
      exporting: false,
      importing: false,
      exportResult: {
        fileName: "export.rczip",
        sizeBytes: 2048,
        manifest: {
          version: "1",
          exportedAt: "2026-07-17T00:00:00Z",
          productVersion: "v1",
          includes: { database: true, settings: true },
          databaseSize: 1024,
          settingCount: 2,
        },
      },
      exports: [],
      onExportPasswordChange: noop,
      onImportPasswordChange: noop,
      onImportFileNameChange: noop,
      onExport: noop,
      onImport: noop,
    });

    expect(proxy).toBeTruthy();
    expect(backup).toBeTruthy();
    expect(exportImport).toBeTruthy();
  });
});
