import { memo, useEffect, useMemo, useState } from "react";
import { systemApi } from "@/api/system";
import { formatBytes } from "@/lib/format";
import type {
  AuditLogItem,
  ChannelHealthScheduleConfig,
  ExportResult,
  NetworkProxyConfig,
  PortCheckResult,
  ProxyTestResult,
  SchedulerStatus,
  StatusPayload,
  SyncScheduleConfig,
  SystemBackup,
  SystemSetting,
  VersionCheckResult,
} from "@/types";
import { SiteSchedules } from "@/components/settings/SiteSchedules";
import { SettingsBackup } from "@/components/settings/SettingsBackup";
import {
  SettingsAboutCard,
  SettingsAuditLogCard,
  SettingsChannelHealthScheduleCard,
  SettingsHelpCard,
  SettingsJsonEditor,
  SettingsLegendCard,
  SettingsPathCard,
  SettingsPortCheckCard,
  SettingsSchedulerCard,
  SettingsSyncScheduleCard,
  SettingsVersionCheckCard,
} from "@/components/settings/SettingsCards";
import { SettingsExportImport } from "@/components/settings/SettingsExportImport";
import { SettingsProxy } from "@/components/settings/SettingsProxy";

const DEFAULT_PROXY_CONFIG: NetworkProxyConfig = {
  enabled: false,
  url: "http://127.0.0.1:7897",
  bypassLocal: true,
};
const DEFAULT_SYNC_SCHEDULE: SyncScheduleConfig = {
  enabled: true,
  intervalMinutes: 30,
  mode: "local-newapi",
  runOnStartup: false,
};
const DEFAULT_CHANNEL_HEALTH_SCHEDULE: ChannelHealthScheduleConfig = {
  enabled: true,
  intervalMinutes: 60,
  runOnStartup: false,
  limit: 20,
  onlyRisky: false,
};

type BusyState = "" | "backup" | "restore" | "settings" | "proxy" | "delete";

function parseSetting<T>(settings: SystemSetting[], key: string, fallback: T): T {
  const setting = settings.find((item) => item.key === key);
  if (!setting) return fallback;
  try {
    return { ...fallback, ...(JSON.parse(setting.valueJson) as Partial<T>) };
  } catch {
    return fallback;
  }
}

function parseStringSetting(setting: SystemSetting | undefined) {
  if (!setting) return "";
  try {
    const parsed = JSON.parse(setting.valueJson);
    return typeof parsed === "string" ? parsed : "";
  } catch {
    return setting.valueJson?.replace(/^"|"$/g, "") || "";
  }
}

function SettingsBase({
  status,
  onDone,
  dialogEpoch = 0,
}: {
  status: StatusPayload;
  onDone: () => void;
  dialogEpoch?: number;
}) {
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [backups, setBackups] = useState<SystemBackup[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLogItem[]>([]);
  const [scheduler, setScheduler] = useState<SchedulerStatus | null>(status.scheduler || null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState<BusyState>("");
  const [proxyTestTarget, setProxyTestTarget] = useState("https://wxls.ccwu.cc/");
  const [proxyTestResult, setProxyTestResult] = useState<ProxyTestResult | null>(null);
  const [multiSelectBackups, setMultiSelectBackups] = useState(false);
  const [selectedBackups, setSelectedBackups] = useState<string[]>([]);
  const [showHelpGuide, setShowHelpGuide] = useState(false);
  const [portCheckPort, setPortCheckPort] = useState(String(status.port || 3001));
  const [portCheckResult, setPortCheckResult] = useState<PortCheckResult | null>(null);
  const [portChecking, setPortChecking] = useState(false);
  const [versionCheckResult, setVersionCheckResult] = useState<VersionCheckResult | null>(null);
  const [versionChecking, setVersionChecking] = useState(false);
  const [versionCheckURL, setVersionCheckURL] = useState("");
  const [exportPassword, setExportPassword] = useState("");
  const [exportResult, setExportResult] = useState<ExportResult | null>(null);
  const [exporting, setExporting] = useState(false);
  const [importPassword, setImportPassword] = useState("");
  const [importFileName, setImportFileName] = useState("");
  const [importing, setImporting] = useState(false);
  const [exports, setExports] = useState<ExportResult[]>([]);

  const totalBackupSize = backups.reduce((sum, backup) => sum + backup.sizeBytes, 0);
  const proxyConfig = useMemo(() => parseSetting(settings, "network.proxy", DEFAULT_PROXY_CONFIG), [settings]);
  const syncSchedule = useMemo(() => parseSetting(settings, "sync.schedule", DEFAULT_SYNC_SCHEDULE), [settings]);
  const channelHealthSchedule = useMemo(
    () => parseSetting(settings, "channel.health.schedule", DEFAULT_CHANNEL_HEALTH_SCHEDULE),
    [settings],
  );
  const checkinJob = scheduler?.jobs.find((job) => job.key === "checkin.daily");
  const currentVersionCheckURL = useMemo(
    () => parseStringSetting(settings.find((item) => item.key === "app.version_check_url")),
    [settings],
  );

  function upsertSetting(key: string, valueJson: string) {
    setSettings((current) => {
      const existingIndex = current.findIndex((item) => item.key === key);
      if (existingIndex === -1) {
        return [...current, { key, valueJson, updatedAt: new Date().toISOString() }].sort((a, b) =>
          a.key.localeCompare(b.key),
        );
      }
      const next = [...current];
      next[existingIndex] = { ...next[existingIndex], valueJson };
      return next;
    });
  }

  function updateProxyConfig(patch: Partial<NetworkProxyConfig>) {
    upsertSetting("network.proxy", JSON.stringify({ ...proxyConfig, ...patch }));
    setProxyTestResult(null);
  }

  function updateSyncSchedule(patch: Partial<SyncScheduleConfig>) {
    upsertSetting("sync.schedule", JSON.stringify({ ...syncSchedule, ...patch }));
  }

  function updateChannelHealthSchedule(patch: Partial<ChannelHealthScheduleConfig>) {
    upsertSetting("channel.health.schedule", JSON.stringify({ ...channelHealthSchedule, ...patch }));
  }

  function toggleBackupSelection(fileName: string) {
    setSelectedBackups((current) =>
      current.includes(fileName) ? current.filter((item) => item !== fileName) : [...current, fileName],
    );
  }

  /** 并行加载设置页只读数据；exports 失败时降级为空数组。 */
  async function refresh() {
    try {
      const [nextSettings, nextBackups, nextScheduler, nextAuditLogs, nextExports] = await Promise.all([
        systemApi.listSettings(),
        systemApi.listBackups(),
        systemApi.schedulerStatus(),
        systemApi.auditLog(),
        systemApi.listExports().catch(() => []),
      ]);
      setSettings(nextSettings);
      setBackups(nextBackups);
      setScheduler(nextScheduler);
      setAuditLogs(nextAuditLogs);
      setExports(nextExports || []);
    } catch (err) {
      setMessage(err instanceof Error ? `加载设置失败：${err.message}` : "加载设置失败");
    }
  }

  async function createBackup() {
    setBusy("backup");
    setMessage("正在创建数据库备份…");
    try {
      const backup = await systemApi.createBackup();
      setMessage("备份完成：" + backup.fileName);
      await refresh();
      onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "备份失败");
    } finally {
      setBusy("");
    }
  }

  async function restoreBackup(backup: SystemBackup) {
    if (
      !window.confirm(
        "确认从 " + backup.fileName + " 恢复数据库？程序会先自动备份当前数据库，然后恢复该快照。恢复后建议刷新页面。",
      )
    )
      return;
    setBusy("restore");
    setMessage("正在恢复 " + backup.fileName + "…");
    try {
      // 恢复语义保持不变：仅提交 fileName，确认框仍由 UI 持有。
      const result = await systemApi.restoreBackup(backup.fileName);
      setMessage("已恢复 " + result.fileName + "，恢复前快照已保存为 " + result.beforeBackup.fileName + "。");
      await refresh();
      onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "恢复失败");
    } finally {
      setBusy("");
    }
  }

  async function deleteSelectedBackups() {
    if (!selectedBackups.length) return;
    if (
      !window.confirm(
        "确认删除选中的 " + selectedBackups.length + " 个本地备份？这不会影响当前数据库，但删除后这些快照无法恢复。",
      )
    )
      return;
    setBusy("delete");
    setMessage("正在删除选中的备份…");
    try {
      const result = await systemApi.deleteBackups(selectedBackups);
      setMessage(
        "已删除 " +
          result.deleted +
          " 个备份" +
          (result.skipped.length ? "，跳过 " + result.skipped.length + " 个" : "") +
          "。",
      );
      setSelectedBackups([]);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "删除备份失败");
    } finally {
      setBusy("");
    }
  }

  async function persistSettings(nextSettings = settings) {
    for (const setting of nextSettings) JSON.parse(setting.valueJson);
    const result = await systemApi.saveSettings(
      nextSettings.map((item) => ({ key: item.key, valueJson: item.valueJson })),
    );
    await refresh();
    onDone();
    return result;
  }

  async function saveSettings() {
    setBusy("settings");
    setMessage("正在保存系统设置…");
    try {
      const result = await persistSettings();
      setMessage("已保存 " + result.updated + " 项设置。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "设置 JSON 格式不正确");
    } finally {
      setBusy("");
    }
  }

  async function testProxy() {
    setBusy("proxy");
    setMessage("正在保存并测试代理…");
    setProxyTestResult(null);
    try {
      await persistSettings();
      const result = await systemApi.proxyTest(proxyTestTarget);
      setProxyTestResult(result);
      setMessage(result.ok ? "代理测试通过：" + result.message : "代理测试失败：" + result.message);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "代理测试失败");
    } finally {
      setBusy("");
    }
  }

  async function checkVersion() {
    setVersionChecking(true);
    setVersionCheckResult(null);
    try {
      if (versionCheckURL !== currentVersionCheckURL) {
        upsertSetting("app.version_check_url", JSON.stringify(versionCheckURL));
        await systemApi.saveSettings([{ key: "app.version_check_url", valueJson: JSON.stringify(versionCheckURL) }]);
      }
      const result = await systemApi.versionCheck();
      setVersionCheckResult(result);
    } catch (error) {
      setVersionCheckResult({
        currentVersion: status.productVersion,
        updateAvailable: false,
        checkedAt: new Date().toISOString(),
        error: error instanceof Error ? error.message : "检查失败",
      });
    } finally {
      setVersionChecking(false);
    }
  }

  async function checkPort() {
    setPortChecking(true);
    setPortCheckResult(null);
    try {
      const result = await systemApi.portCheck(portCheckPort);
      setPortCheckResult(result);
    } catch {
      setPortCheckResult({ port: Number(portCheckPort) || 0, available: false, inUse: false, error: "检测失败" });
    } finally {
      setPortChecking(false);
    }
  }

  async function exportDatabase() {
    setExporting(true);
    setExportResult(null);
    try {
      const result = await systemApi.exportDatabase(exportPassword);
      setExportResult(result);
      setExportPassword("");
      setMessage("加密导出成功");
      setExports((await systemApi.listExports()) || []);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "导出失败");
    } finally {
      setExporting(false);
    }
  }

  async function importDatabase() {
    if (!window.confirm("导入将覆盖当前数据库，确定继续？")) return;
    setImporting(true);
    try {
      await systemApi.importDatabase(importPassword, importFileName);
      setMessage("导入成功，正在刷新…");
      setTimeout(() => window.location.reload(), 1500);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "导入失败");
    } finally {
      setImporting(false);
    }
  }

  useEffect(() => {
    setShowHelpGuide(false);
  }, [dialogEpoch]);
  useEffect(() => {
    void refresh();
  }, []);
  useEffect(() => {
    setVersionCheckURL(currentVersionCheckURL);
  }, [currentVersionCheckURL]);

  return (
    <section className="panel">
      <div className="settings-hero">
        <div>
          <span className="eyebrow">本地维护</span>
          <h2>本地数据安全与运行配置</h2>
          <p>备份只保存在本机 data/backups 目录。恢复前会自动创建当前数据库快照，避免误操作不可回退。</p>
        </div>
        <button disabled={busy !== ""} onClick={() => void createBackup()}>
          {busy === "backup" ? "备份中…" : "立即备份数据库"}
        </button>
      </div>

      <div className="channel-summary">
        <div>
          <span>运行端口</span>
          <strong>{status.port}</strong>
        </div>
        <div>
          <span>备份数量</span>
          <strong>{backups.length}</strong>
        </div>
        <div>
          <span>备份占用</span>
          <strong>{formatBytes(totalBackupSize)}</strong>
        </div>
        <div>
          <span>未读通知</span>
          <strong>{status.summary.unreadNotifications}</strong>
        </div>
      </div>

      <div className="settings-grid">
        <SettingsAboutCard status={status} scheduler={scheduler} checkinJob={checkinJob} />
        <SettingsVersionCheckCard
          status={status}
          versionCheckURL={versionCheckURL}
          currentVersionCheckURL={currentVersionCheckURL}
          versionChecking={versionChecking}
          versionCheckResult={versionCheckResult}
          onURLChange={setVersionCheckURL}
          onPersistURL={() => upsertSetting("app.version_check_url", JSON.stringify(versionCheckURL))}
          onCheck={() => void checkVersion()}
        />
        <SettingsPortCheckCard
          status={status}
          portCheckPort={portCheckPort}
          portChecking={portChecking}
          portCheckResult={portCheckResult}
          onPortChange={setPortCheckPort}
          onCheck={() => void checkPort()}
        />
        <SettingsPathCard status={status} />
        <SettingsExportImport
          exportPassword={exportPassword}
          importPassword={importPassword}
          importFileName={importFileName}
          exporting={exporting}
          importing={importing}
          exportResult={exportResult}
          exports={exports}
          onExportPasswordChange={setExportPassword}
          onImportPasswordChange={setImportPassword}
          onImportFileNameChange={setImportFileName}
          onExport={() => void exportDatabase()}
          onImport={() => void importDatabase()}
        />
        <SettingsHelpCard showHelpGuide={showHelpGuide} onToggle={() => setShowHelpGuide((current) => !current)} />
        <SettingsLegendCard />
        <SettingsProxy
          proxyConfig={proxyConfig}
          proxyTestTarget={proxyTestTarget}
          proxyTestResult={proxyTestResult}
          busy={busy === "proxy"}
          canSave={Boolean(settings.length)}
          defaultConfig={DEFAULT_PROXY_CONFIG}
          onPatch={updateProxyConfig}
          onTargetChange={setProxyTestTarget}
          onTest={() => void testProxy()}
          onReset={() => updateProxyConfig(DEFAULT_PROXY_CONFIG)}
        />
        <SettingsSyncScheduleCard
          syncSchedule={syncSchedule}
          busy={busy === "settings"}
          canSave={Boolean(settings.length)}
          onPatch={updateSyncSchedule}
          onSave={() => void saveSettings()}
        />
        <SettingsChannelHealthScheduleCard
          channelHealthSchedule={channelHealthSchedule}
          busy={busy === "settings"}
          canSave={Boolean(settings.length)}
          defaultConfig={DEFAULT_CHANNEL_HEALTH_SCHEDULE}
          onPatch={updateChannelHealthSchedule}
          onSave={() => void saveSettings()}
        />
        <SettingsSchedulerCard scheduler={scheduler} busy={busy !== ""} onRefresh={() => void refresh()} />
        <SiteSchedules />
        <SettingsAuditLogCard auditLogs={auditLogs} busy={busy !== ""} onRefresh={() => void refresh()} />
        <SettingsBackup
          backups={backups}
          busy={busy === "delete" || busy === "restore"}
          multiSelect={multiSelectBackups}
          selected={selectedBackups}
          onRefresh={() => void refresh()}
          onToggleMulti={() => setMultiSelectBackups((current) => !current)}
          onToggleSelect={toggleBackupSelection}
          onDeleteSelected={() => void deleteSelectedBackups()}
          onRestore={(backup) => void restoreBackup(backup)}
        />
      </div>

      <SettingsJsonEditor
        settings={settings}
        busy={busy === "settings"}
        onSave={() => void saveSettings()}
        onChange={setSettings}
      />
      {message ? <div className="note">{message}</div> : null}
    </section>
  );
}

export const Settings = memo(SettingsBase);
