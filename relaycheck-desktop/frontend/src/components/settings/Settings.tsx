import { memo, useEffect, useMemo, useState } from "react";
import { api } from "@/api/client";
import { reopenOnboarding } from "@/components/onboarding/OnboardingWizard";
import { formatBuildTime, formatBytes, formatTime } from "@/lib/format";
import { auditActionLabel, auditLevelLabel, diagnosticLevelLabel, schedulerStatusLabel } from "@/lib/labels";
import type { AuditLogItem, ChannelHealthScheduleConfig, ExportResult, NetworkProxyConfig, PortCheckResult, ProxyTestResult, SchedulerStatus, StatusPayload, SyncScheduleConfig, SystemBackup, SystemSetting, VersionCheckResult } from "@/types";
import { EmptyState } from "@/components/ui/empty-state";
import { StatusLabel } from "@/components/ui/status-label";
import { SiteSchedules } from "@/components/settings/SiteSchedules";

function SettingsBase({ status, onDone }: { status: StatusPayload; onDone: () => void }) {
  const [settings, setSettings] = useState<SystemSetting[]>([]);
  const [backups, setBackups] = useState<SystemBackup[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLogItem[]>([]);
  const [scheduler, setScheduler] = useState<SchedulerStatus | null>(status.scheduler || null);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState<"" | "backup" | "restore" | "settings" | "proxy" | "delete">("");
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
  const [exports, setExports] = useState<ExportResult[]>([]);
  const [importing, setImporting] = useState(false);
  const totalBackupSize = backups.reduce((sum, backup) => sum + backup.sizeBytes, 0);
  const defaultProxyConfig: NetworkProxyConfig = { enabled: false, url: "http://127.0.0.1:7897", bypassLocal: true };
  const defaultSyncSchedule: SyncScheduleConfig = { enabled: true, intervalMinutes: 30, mode: "local-newapi", runOnStartup: false };
  const defaultChannelHealthSchedule: ChannelHealthScheduleConfig = { enabled: true, intervalMinutes: 60, runOnStartup: false, limit: 20, onlyRisky: false };
  const proxyConfig = useMemo(() => {
    const setting = settings.find((item) => item.key === "network.proxy");
    if (!setting) return defaultProxyConfig;
    try {
      return { ...defaultProxyConfig, ...(JSON.parse(setting.valueJson) as Partial<NetworkProxyConfig>) };
    } catch {
      return defaultProxyConfig;
    }
  }, [settings]);
  const syncSchedule = useMemo(() => {
    const setting = settings.find((item) => item.key === "sync.schedule");
    if (!setting) return defaultSyncSchedule;
    try {
      return { ...defaultSyncSchedule, ...(JSON.parse(setting.valueJson) as Partial<SyncScheduleConfig>) };
    } catch {
      return defaultSyncSchedule;
    }
  }, [settings]);
  const channelHealthSchedule = useMemo(() => {
    const setting = settings.find((item) => item.key === "channel.health.schedule");
    if (!setting) return defaultChannelHealthSchedule;
    try {
      return { ...defaultChannelHealthSchedule, ...(JSON.parse(setting.valueJson) as Partial<ChannelHealthScheduleConfig>) };
    } catch {
      return defaultChannelHealthSchedule;
    }
  }, [settings]);
  const checkinJob = scheduler?.jobs.find((job) => job.key === "checkin.daily");
  const syncJob = scheduler?.jobs.find((job) => job.key === "sync.local_newapi");
  const channelHealthJob = scheduler?.jobs.find((job) => job.key === "channel.health_probe");
  const versionCheckURLSetting = settings.find((item) => item.key === "app.version_check_url");
  const currentVersionCheckURL = useMemo(() => {
    if (!versionCheckURLSetting) return "";
    try {
      const parsed = JSON.parse(versionCheckURLSetting.valueJson);
      return typeof parsed === "string" ? parsed : "";
    } catch {
      return versionCheckURLSetting.valueJson?.replace(/^"|"$/g, "") || "";
    }
  }, [versionCheckURLSetting]);

  function upsertSetting(key: string, valueJson: string) {
    setSettings((current) => {
      const existingIndex = current.findIndex((item) => item.key === key);
      if (existingIndex === -1) {
        return [...current, { key, valueJson, updatedAt: new Date().toISOString() }].sort((a, b) => a.key.localeCompare(b.key));
      }
      const next = [...current];
      next[existingIndex] = { ...next[existingIndex], valueJson };
      return next;
    });
  }

  function updateProxyConfig(patch: Partial<NetworkProxyConfig>) {
    const nextConfig = { ...proxyConfig, ...patch };
    upsertSetting("network.proxy", JSON.stringify(nextConfig));
    setProxyTestResult(null);
  }

  function updateSyncSchedule(patch: Partial<SyncScheduleConfig>) {
    const nextConfig = { ...syncSchedule, ...patch };
    upsertSetting("sync.schedule", JSON.stringify(nextConfig));
  }

  function updateChannelHealthSchedule(patch: Partial<ChannelHealthScheduleConfig>) {
    const nextConfig = { ...channelHealthSchedule, ...patch };
    upsertSetting("channel.health.schedule", JSON.stringify(nextConfig));
  }

  function toggleBackupSelection(fileName: string) {
    setSelectedBackups((current) => current.includes(fileName) ? current.filter((item) => item !== fileName) : [...current, fileName]);
  }

  async function refresh() {
    try {
      const [nextSettings, nextBackups, nextScheduler, nextAuditLogs, nextExports] = await Promise.all([
        api<SystemSetting[]>("/api/system/settings"),
        api<SystemBackup[]>("/api/system/backups"),
        api<SchedulerStatus>("/api/system/scheduler-status"),
        api<AuditLogItem[]>("/api/system/audit-log"),
        api<ExportResult[]>("/api/system/exports").catch(() => []),
      ]);
      setSettings(nextSettings);
      setBackups(nextBackups);
      setScheduler(nextScheduler);
      setAuditLogs(nextAuditLogs);
      setExports(nextExports || []);
    } catch (err) {
      // refresh() is called via `void` on mount and after backup/restore
      // operations; without this catch a failure of any of the four
      // unguarded api() calls surfaces as an unhandled rejection and the
      // panel silently keeps stale data with no error message.
      setMessage(err instanceof Error ? `加载设置失败：${err.message}` : "加载设置失败");
    }
  }

  async function createBackup() {
    setBusy("backup");
    setMessage("正在创建数据库备份…");
    try {
      const backup = await api<SystemBackup>("/api/system/backup", { method: "POST" });
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
    const confirmed = window.confirm("确认从 " + backup.fileName + " 恢复数据库？程序会先自动备份当前数据库，然后恢复该快照。恢复后建议刷新页面。");
    if (!confirmed) return;
    setBusy("restore");
    setMessage("正在恢复 " + backup.fileName + "…");
    try {
      const result = await api<{ restored: boolean; fileName: string; beforeBackup: SystemBackup }>("/api/system/restore", {
        method: "POST",
        body: JSON.stringify({ fileName: backup.fileName }),
      });
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
    const confirmed = window.confirm("确认删除选中的 " + selectedBackups.length + " 个本地备份？这不会影响当前数据库，但删除后这些快照无法恢复。");
    if (!confirmed) return;
    setBusy("delete");
    setMessage("正在删除选中的备份…");
    try {
      const result = await api<{ deleted: number; skipped: string[] }>("/api/system/backups/delete", {
        method: "POST",
        body: JSON.stringify({ fileNames: selectedBackups }),
      });
      setMessage("已删除 " + result.deleted + " 个备份" + (result.skipped.length ? "，跳过 " + result.skipped.length + " 个" : "") + "。");
      setSelectedBackups([]);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "删除备份失败");
    } finally {
      setBusy("");
    }
  }

  async function persistSettings(nextSettings = settings) {
    for (const setting of nextSettings) {
      JSON.parse(setting.valueJson);
    }
    const result = await api<{ updated: number }>("/api/system/settings", {
      method: "PUT",
      body: JSON.stringify({ settings: nextSettings }),
    });
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
      const result = await api<ProxyTestResult>("/api/system/proxy-test", {
        method: "POST",
        body: JSON.stringify({ targetUrl: proxyTestTarget }),
      });
      setProxyTestResult(result);
      setMessage(result.ok ? "代理测试通过：" + result.message : "代理测试失败：" + result.message);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "代理测试失败");
    } finally {
      setBusy("");
    }
  }

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
        <article className="card settings-about-card">
          <div className="section-heading">
            <div>
              <strong>关于 / 版本</strong>
              <span>{status.productName} &middot; {status.productVersion}</span>
            </div>
            <span className="status-pill success"><StatusLabel level="success" label="正式版" /></span>
          </div>
          <div className="detail-list">
            <div><span>显示名</span><strong>{status.productName}</strong></div>
            <div><span>版本</span><strong>{status.productVersion}</strong></div>
            <div><span>构建时间</span><strong>{formatBuildTime(status.buildTime)}</strong></div>
            <div><span>绑定地址</span><strong>{status.bindAddress}:{status.port}</strong></div>
            {status.portConflict && status.preferredPort ? (
              <div className="warning-banner" style={{ marginTop: 8, padding: "8px 12px", borderRadius: 8, fontSize: 13 }}>
                <span>端口冲突</span>
                <strong>首选端口 {status.preferredPort} 被占用，已回退到 {status.port}</strong>
              </div>
            ) : null}
            <div><span>调度器</span><strong>{scheduler ? `${scheduler.jobs.length} 个任务 &middot; ${schedulerStatusLabel(checkinJob?.status || "idle")}` : "读取中"}</strong></div>
            <div>
              <span>上次自检</span>
              <strong>{status.lastDiagnostics ? `${diagnosticLevelLabel(status.lastDiagnostics.overall)} &middot; ${status.lastDiagnostics.itemCount} 项 &middot; ${formatTime(status.lastDiagnostics.generatedAt)}` : "未生成"}</strong>
            </div>
          </div>
        </article>

        <article className="card settings-version-check-card">
          <div className="section-heading">
            <div>
              <strong>版本检查</strong>
              <span>检查是否有新版本可用</span>
            </div>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>版本清单 URL</span>
              <input
                value={versionCheckURL}
                onChange={(event) => setVersionCheckURL(event.target.value)}
                onBlur={() => {
                  if (versionCheckURL !== currentVersionCheckURL) {
                    upsertSetting("app.version_check_url", JSON.stringify(versionCheckURL));
                  }
                }}
                placeholder="https://example.com/relaycheck-version.json"
              />
            </label>
            <button
              type="button"
              disabled={versionChecking}
              onClick={async () => {
                setVersionChecking(true);
                setVersionCheckResult(null);
                try {
                  if (versionCheckURL !== currentVersionCheckURL) {
                    upsertSetting("app.version_check_url", JSON.stringify(versionCheckURL));
                    await api("/api/system/settings", {
                      method: "PUT",
                      body: JSON.stringify({ settings: [{ key: "app.version_check_url", valueJson: JSON.stringify(versionCheckURL) }] }),
                    });
                  }
                  const result = await api<VersionCheckResult>("/api/system/version-check");
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
              }}
            >
              {versionChecking ? "检查中…" : "检查更新"}
            </button>
          </div>
          {versionCheckResult ? (
            <div className="detail-list" style={{ marginTop: 8 }}>
              <div><span>当前版本</span><strong>{versionCheckResult.currentVersion}</strong></div>
              {versionCheckResult.latestVersion ? (
                <div><span>最新版本</span><strong>{versionCheckResult.latestVersion}</strong></div>
              ) : null}
              <div>
                <span>状态</span>
                <strong>
                  {versionCheckResult.error
                    ? versionCheckResult.error
                    : versionCheckResult.updateAvailable
                      ? "有新版本可用"
                      : "已是最新版本"}
                </strong>
              </div>
              {versionCheckResult.updateAvailable && versionCheckResult.releaseUrl ? (
                <div>
                  <span>下载</span>
                  <strong>
                    <a href={versionCheckResult.releaseUrl} target="_blank" rel="noopener noreferrer" style={{ color: "var(--v4-blue)" }}>
                      打开下载页面
                    </a>
                  </strong>
                </div>
              ) : null}
              {versionCheckResult.releaseNotes ? (
                <div style={{ marginTop: 4, padding: "8px 12px", background: "var(--v4-neutral-bg)", borderRadius: 8, fontSize: 13, whiteSpace: "pre-wrap" }}>
                  {versionCheckResult.releaseNotes}
                </div>
              ) : null}
            </div>
          ) : null}
          <div className="problem-hint detail-hint">
            配置版本清单 URL 后，可检查远程是否有新版本。清单格式: {"{ \"version\": \"v1.1\", \"releaseUrl\": \"...\", \"releaseNotes\": \"...\" }"}
          </div>
        </article>

        <article className="card settings-port-check-card">
          <div className="section-heading">
            <div>
              <strong>端口检测</strong>
              <span>检查本地端口是否可绑定</span>
            </div>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>端口号</span>
              <input
                value={portCheckPort}
                onChange={(event) => setPortCheckPort(event.target.value)}
                placeholder="如 3001"
              />
            </label>
            <button
              type="button"
              disabled={portChecking}
              onClick={async () => {
                setPortChecking(true);
                setPortCheckResult(null);
                try {
                  const result = await api<PortCheckResult>(`/api/system/port-check?port=${encodeURIComponent(portCheckPort)}`);
                  setPortCheckResult(result);
                } catch {
                  setPortCheckResult({ port: Number(portCheckPort) || 0, available: false, inUse: false, error: "检测失败" });
                } finally {
                  setPortChecking(false);
                }
              }}
            >
              {portChecking ? "检测中…" : "检测端口"}
            </button>
          </div>
          {portCheckResult ? (
            <div className="detail-list">
              <div>
                <span>端口</span>
                <strong>{portCheckResult.port}</strong>
              </div>
              <div>
                <span>状态</span>
                <strong>
                  {portCheckResult.available
                    ? "可用（未被占用）"
                    : portCheckResult.inUse
                      ? "已被占用"
                      : "检测失败"}
                </strong>
              </div>
              {portCheckResult.error ? (
                <div><span>详情</span><strong>{portCheckResult.error}</strong></div>
              ) : null}
            </div>
          ) : null}
          <div className="problem-hint detail-hint">
            启动前检测端口可避免端口冲突。当前运行端口为 {status.port}。
          </div>
        </article>

        <article className="card settings-path-card">
          <strong>本地路径</strong>
          <div className="detail-list">
            <div><span>数据库</span><strong>{status.databasePath}</strong></div>
            <div><span>备份目录</span><strong>{status.backupDir}</strong></div>
            <div><span>架构</span><strong>{status.architecture}</strong></div>
            <div><span>代理</span><strong>{status.networkProxy?.enabled ? status.networkProxy.urlMasked : "未启用"}</strong></div>
          </div>
          <div className="problem-hint detail-hint">建议在大量导入、批量识别、批量签到前先点一次"立即备份数据库"。</div>
        </article>

        <article className="card settings-export-card">
          <div className="section-heading">
            <div>
              <strong>加密导出 / 导入</strong>
              <span>将渠道、凭据、历史和设置打包为 AES-GCM 加密文件</span>
            </div>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>导出密码（至少 6 位）</span>
              <input
                type="password"
                value={exportPassword}
                onChange={(e) => setExportPassword(e.target.value)}
                placeholder="设置导出密码"
              />
            </label>
            <button
              type="button"
              disabled={exporting || exportPassword.length < 6}
              onClick={async () => {
                setExporting(true);
                setExportResult(null);
                try {
                  const result = await api<ExportResult>("/api/system/export", {
                    method: "POST",
                    body: JSON.stringify({ password: exportPassword }),
                  });
                  setExportResult(result);
                  setMessage("加密导出成功");
                  const list = await api<ExportResult[]>("/api/system/exports");
                  setExports(list || []);
                } catch (error) {
                  setMessage(error instanceof Error ? error.message : "导出失败");
                } finally {
                  setExporting(false);
                }
              }}
            >
              {exporting ? "导出中…" : "加密导出"}
            </button>
          </div>
          {exportResult ? (
            <div className="detail-list" style={{ marginTop: 8 }}>
              <div><span>文件名</span><strong>{exportResult.fileName}</strong></div>
              <div><span>大小</span><strong>{formatBytes(exportResult.sizeBytes)}</strong></div>
              <div><span>数据库</span><strong>{formatBytes(exportResult.manifest.databaseSize)}</strong></div>
              <div><span>设置数</span><strong>{exportResult.manifest.settingCount}</strong></div>
              <div><span>导出时间</span><strong>{formatTime(exportResult.manifest.exportedAt)}</strong></div>
            </div>
          ) : null}
          {exports.length > 0 ? (
            <div className="detail-list" style={{ marginTop: 12 }}>
              <div className="section-heading"><strong>已有导出文件</strong></div>
              {exports.map((exp) => (
                <div key={exp.fileName}>
                  <span>{exp.fileName}</span>
                  <strong>
                    {formatBytes(exp.sizeBytes)}
                    <button
                      type="button"
                      className="ghost"
                      style={{ marginLeft: 8, padding: "2px 8px", fontSize: 12 }}
                      onClick={() => { setImportFileName(exp.fileName); }}
                    >
                      选择导入
                    </button>
                  </strong>
                </div>
              ))}
            </div>
          ) : null}
          <div className="proxy-form-grid" style={{ marginTop: 12 }}>
            <label className="field">
              <span>导入密码</span>
              <input
                type="password"
                value={importPassword}
                onChange={(e) => setImportPassword(e.target.value)}
                placeholder="输入导出时设置的密码"
              />
            </label>
            <label className="field">
              <span>导入文件</span>
              <input
                value={importFileName}
                onChange={(e) => setImportFileName(e.target.value)}
                placeholder="export-XXXXXXXX-XXXXXX.rczip"
              />
            </label>
            <button
              type="button"
              className="danger"
              disabled={importing || importPassword.length < 6 || !importFileName}
              onClick={async () => {
                if (!confirm("导入将覆盖当前数据库，确定继续？")) return;
                setImporting(true);
                try {
                  await api("/api/system/import", {
                    method: "POST",
                    body: JSON.stringify({ password: importPassword, fileName: importFileName }),
                  });
                  setMessage("导入成功，正在刷新…");
                  setTimeout(() => window.location.reload(), 1500);
                } catch (error) {
                  setMessage(error instanceof Error ? error.message : "导入失败");
                } finally {
                  setImporting(false);
                }
              }}
            >
              {importing ? "导入中…" : "加密导入"}
            </button>
          </div>
          <div className="problem-hint detail-hint">
            导出文件使用 AES-256-GCM 加密，包含完整数据库和所有设置。导入会覆盖当前数据，请先备份。
          </div>
        </article>

        <article className="card settings-help-card">
          <div className="section-heading">
            <div>
              <strong>帮助 / 文档</strong>
              <span>把常用说明集中在本地设置页，避免需要翻目录才知道下一步。</span>
            </div>
            <div className="toolbar compact-toolbar">
              <button className="ghost" type="button" onClick={() => setShowHelpGuide((current) => !current)}>
                {showHelpGuide ? "收起" : "查看指引"}
              </button>
              <button className="ghost" type="button" onClick={reopenOnboarding}>
                重新查看引导
              </button>
            </div>
          </div>
          <div className="detail-list">
            <div><span>使用说明</span><strong>relaycheck-desktop/README.md</strong></div>
            <div><span>总清单</span><strong>relaycheck-desktop/PROMPT_CHECKLIST.md</strong></div>
            <div><span>设计规则</span><strong>relaycheck-desktop/DESIGN_SYSTEM.md</strong></div>
            <div><span>接力说明</span><strong>relaycheck-desktop/AGENT_HANDOFF.md</strong></div>
          </div>
          {showHelpGuide ? (
            <div className="detail-stack">
              <div className="problem-hint detail-hint">新手路径：先去"本机扫描"导入 NewAPI，再到"账号"补授权或 API Key，最后在"签到"和"余额"验证一次。</div>
              <div className="note">遇到异常优先看"总览"的处理建议中心；做批量操作前先在本页创建数据库备份。</div>
            </div>
          ) : null}
        </article>

        <article className="card settings-legend-card">
          <div className="section-heading">
            <div>
              <strong>能力图例</strong>
              <span>常驻解释后台、Key、模型和价格 chip，减少状态只靠颜色判断。</span>
            </div>
          </div>
          <div className="chips">
            <span>NEW = NewAPI</span>
            <span>ONE = OneAPI</span>
            <span>SUB = Sub2API</span>
            <span>MOD = 魔改中转</span>
          </div>
          <div className="detail-list">
            <div><span>Key 有效</span><strong>已读取 /v1/models 且密钥可用</strong></div>
            <div><span>模型可用</span><strong>最小 chat completion 测试通过</strong></div>
            <div><span>raw_json</span><strong>来自 NewAPI 渠道原始配置的回退识别</strong></div>
            <div><span>live</span><strong>使用渠道 Key 实时请求上游模型列表</strong></div>
          </div>
        </article>

        <article className="card settings-proxy-card">
          <div className="section-heading">
            <div>
              <strong>网络代理</strong>
              <span>用于外部中转站探测、签到、余额刷新和 API Key 检测。本地 127.0.0.1 默认直连。</span>
            </div>
            <span className={"status-pill " + (proxyConfig.enabled ? "success" : "neutral")}>
              <StatusLabel level={proxyConfig.enabled ? "enabled" : "disabled"} label={proxyConfig.enabled ? "已启用" : "未启用"} />
            </span>
          </div>
          <div className="proxy-toggle-row">
            <label className="check">
              <input type="checkbox" checked={proxyConfig.enabled} onChange={(event) => updateProxyConfig({ enabled: event.target.checked })} />
              启用代理
            </label>
            <label className="check">
              <input type="checkbox" checked={proxyConfig.bypassLocal} onChange={(event) => updateProxyConfig({ bypassLocal: event.target.checked })} />
              绕过本地地址
            </label>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>代理地址</span>
              <input value={proxyConfig.url} onChange={(event) => updateProxyConfig({ url: event.target.value })} placeholder="http://127.0.0.1:7897" />
            </label>
            <label className="field">
              <span>测试地址</span>
              <input value={proxyTestTarget} onChange={(event) => setProxyTestTarget(event.target.value)} placeholder="https://wxls.ccwu.cc/" />
            </label>
          </div>
          <div className="proxy-actions">
            <button disabled={busy !== "" || !settings.length} onClick={() => void testProxy()}>
              {busy === "proxy" ? "测试中…" : "保存并测试代理"}
            </button>
            <button className="ghost" disabled={busy !== ""} onClick={() => updateProxyConfig(defaultProxyConfig)}>恢复默认</button>
          </div>
          {proxyTestResult ? (
            <div className={"proxy-result " + (proxyTestResult.ok ? "success" : "warning")}>
              <strong><StatusLabel level={proxyTestResult.ok ? "success" : "warning"} label={proxyTestResult.ok ? "连通" : "未连通"} /></strong>
              <span>{proxyTestResult.targetUrl} {"·"} {proxyTestResult.httpStatus ? "HTTP " + proxyTestResult.httpStatus + " · " : ""}{proxyTestResult.latencyMs}ms</span>
              <p>{proxyTestResult.message}</p>
            </div>
          ) : (
            <div className="problem-hint detail-hint">如果某些站点 Chrome 能打开但工具检测失败，先开启这里的代理并测试目标站点。</div>
          )}
        </article>

        <article className="card settings-sync-card">
          <div className="section-heading">
            <div>
              <strong>同步频率</strong>
              <span>默认每 30 分钟同步一次本地 NewAPI 数据；后台调度器会读取这里的配置。</span>
            </div>
            <span className={"status-pill " + (syncSchedule.enabled ? "success" : "neutral")}>
              <StatusLabel level={syncSchedule.enabled ? "enabled" : "disabled"} label={syncSchedule.enabled ? "已启用" : "未启用"} />
            </span>
          </div>
          <div className="proxy-toggle-row">
            <label className="check">
              <input type="checkbox" checked={syncSchedule.enabled} onChange={(event) => updateSyncSchedule({ enabled: event.target.checked })} />
              启用定时同步
            </label>
            <label className="check">
              <input type="checkbox" checked={syncSchedule.runOnStartup} onChange={(event) => updateSyncSchedule({ runOnStartup: event.target.checked })} />
              启动后同步一次
            </label>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>同步间隔（分钟）</span>
              <input type="number" min={5} max={1440} value={syncSchedule.intervalMinutes}
                onChange={(event) => updateSyncSchedule({ intervalMinutes: Math.max(5, Number(event.target.value) || 30) })} />
            </label>
            <label className="field">
              <span>同步模式</span>
              <select value={syncSchedule.mode} onChange={(event) => updateSyncSchedule({ mode: event.target.value })}>
                <option value="local-newapi">本地 NewAPI 实例</option>
                <option value="manual-only">只手动同步</option>
              </select>
            </label>
          </div>
          <div className="problem-hint detail-hint">后台同步默认不导入渠道 Key、不做重探测，只更新渠道结构和源端移除状态；失败才发重要通知。</div>
          <div className="proxy-actions">
            <button disabled={busy !== "" || !settings.length} onClick={() => void saveSettings()}>
              {busy === "settings" ? "保存中…" : "保存同步频率"}
            </button>
          </div>
        </article>

        <article className="card settings-sync-card">
          <div className="section-heading">
            <div>
              <strong>渠道健康探测</strong>
              <span>定期刷新中转站识别、站点健康、渠道模型状态，并把异常推送到处理中心。</span>
            </div>
            <span className={"status-pill " + (channelHealthSchedule.enabled ? "success" : "neutral")}>
              <StatusLabel level={channelHealthSchedule.enabled ? "enabled" : "disabled"} label={channelHealthSchedule.enabled ? "已启用" : "未启用"} />
            </span>
          </div>
          <div className="proxy-toggle-row">
            <label className="check">
              <input type="checkbox" checked={channelHealthSchedule.enabled} onChange={(event) => updateChannelHealthSchedule({ enabled: event.target.checked })} />
              启用自动探测
            </label>
            <label className="check">
              <input type="checkbox" checked={channelHealthSchedule.runOnStartup} onChange={(event) => updateChannelHealthSchedule({ runOnStartup: event.target.checked })} />
              启动后立即探测
            </label>
            <label className="check">
              <input type="checkbox" checked={channelHealthSchedule.onlyRisky} onChange={(event) => updateChannelHealthSchedule({ onlyRisky: event.target.checked })} />
              只探测风险站点
            </label>
          </div>
          <div className="proxy-form-grid">
            <label className="field">
              <span>探测间隔（分钟）</span>
              <input type="number" min={5} max={1440} value={channelHealthSchedule.intervalMinutes}
                onChange={(event) => updateChannelHealthSchedule({ intervalMinutes: Math.max(5, Number(event.target.value) || 60) })} />
            </label>
            <label className="field">
              <span>单次站点上限</span>
              <input type="number" min={1} max={50} value={channelHealthSchedule.limit}
                onChange={(event) => updateChannelHealthSchedule({ limit: Math.min(50, Math.max(1, Number(event.target.value) || 20)) })} />
            </label>
          </div>
          <div className="problem-hint detail-hint">调度器会复用渠道页的“探测健康”流程，发现站点不可达、模型同步失败或 Key 状态异常时记录预警。</div>
          <div className="proxy-actions">
            <button disabled={busy !== "" || !settings.length} onClick={() => void saveSettings()}>
              {busy === "settings" ? "保存中…" : "保存健康探测计划"}
            </button>
            <button className="ghost" disabled={busy !== ""} onClick={() => updateChannelHealthSchedule(defaultChannelHealthSchedule)}>恢复默认</button>
          </div>
        </article>

        <article className="card scheduler-card">
          <div className="section-heading">
            <div>
              <strong>后台调度器</strong>
              <span>{scheduler ? ("状态刷新于 " + formatTime(scheduler.generatedAt)) : "读取自动签到和同步运行状态"}</span>
            </div>
            <button className="ghost" disabled={busy !== ""} onClick={() => void refresh()}>刷新</button>
          </div>
          <div className="scheduler-job-grid">
            {[
              { key: "checkin.daily", fallback: "自动签到", job: checkinJob },
              { key: "sync.local_newapi", fallback: "NewAPI 定时同步", job: syncJob },
              { key: "channel.health_probe", fallback: "渠道健康探测", job: channelHealthJob },
            ].map(({ key, fallback, job }) => (
              <article className={"scheduler-job " + (job?.status || "idle")} key={key}>
                <div>
                  <span>{job?.label || fallback}</span>
                  <strong><StatusLabel level={job?.status || "idle"} label={schedulerStatusLabel(job?.status || "idle")} /></strong>
                </div>
                <div className="scheduler-job-meta">
                  <span>下次 {formatTime(job?.nextRunAt || "")}</span>
                  <span>上次 {formatTime(job?.lastFinishedAt || job?.lastStartedAt || "")}</span>
                  {job?.summary ? <span>{job.summary}</span> : null}
                  {job?.lastError ? <span className="danger-text">{job.lastError}</span> : null}
                </div>
              </article>
            ))}
          </div>
        </article>

        <SiteSchedules />

        <article className="card audit-log-card">
          <div className="section-heading">
            <div>
              <strong>审计日志</strong>
              <span>最近 {Math.min(auditLogs.length, 12)} 条安全与维护事件，只读留痕。</span>
            </div>
            <button className="ghost" disabled={busy !== ""} onClick={() => void refresh()}>刷新</button>
          </div>
          <div className="list compact audit-log-list">
            {auditLogs.slice(0, 12).map((item) => (
              <article className={"detail-row audit-row " + item.level} key={item.id}>
                <div>
                  <strong>{auditActionLabel(item.action)}</strong>
                  <span>{item.summary} {"·"} {formatTime(item.createdAt)}</span>
                </div>
                <b><StatusLabel level={item.level} label={auditLevelLabel(item.level)} /></b>
              </article>
            ))}
            {!auditLogs.length ? <EmptyState title="暂无审计记录" description="登录、设置、备份、账号和站点维护会在这里留下只读记录。" /> : null}
          </div>
        </article>

        <article className="card">
          <div className="section-heading">
            <div>
              <strong>备份快照</strong>
              <span>{multiSelectBackups ? ("已选择 " + selectedBackups.length + " 个备份") : "默认突出最新一个；可打开多选清理旧快照。"}</span>
            </div>
            <div className="toolbar compact-toolbar">
              <button className="ghost" disabled={busy !== ""} onClick={() => void refresh()}>刷新</button>
              <button className={multiSelectBackups ? "" : "ghost"} disabled={busy !== ""} onClick={() => setMultiSelectBackups((current) => !current)}>
                {multiSelectBackups ? "退出多选" : "多选管理"}
              </button>
              {multiSelectBackups ? (
                <button className="danger" disabled={busy !== "" || !selectedBackups.length} onClick={() => void deleteSelectedBackups()}>
                  {busy === "delete" ? "删除中…" : "删除选中"}
                </button>
              ) : null}
            </div>
          </div>
          <div className="list compact">
            {(multiSelectBackups ? backups.slice(0, 24) : backups.slice(0, 1)).map((backup, index) => (
              <article className={"detail-row backup-row " + (index === 0 ? "is-latest" : "") + " " + (selectedBackups.includes(backup.fileName) ? "is-selected" : "")} key={backup.fileName}>
                {multiSelectBackups ? (
                  <label className="backup-check">
                    <input type="checkbox" checked={selectedBackups.includes(backup.fileName)} onChange={() => toggleBackupSelection(backup.fileName)} />
                  </label>
                ) : null}
                <div>
                  <strong>{backup.fileName}{index === 0 ? " · 最新" : ""}</strong>
                  <span>{formatBytes(backup.sizeBytes)} {"·"} {formatTime(backup.createdAt)}</span>
                </div>
                <button className="danger" disabled={busy !== ""} onClick={() => void restoreBackup(backup)}>
                  {busy === "restore" ? "恢复中…" : "恢复"}
                </button>
              </article>
            ))}
            {!backups.length ? <EmptyState title="暂无备份" description='点击"立即备份数据库"后，这里会出现可恢复的本地快照。' /> : null}
          </div>
        </article>
      </div>

      <article className="card">
        <div className="section-heading">
          <div>
            <strong>系统设置 JSON</strong>
            <span>轻量保存扫描目标、签到计划和本地运行偏好。保存前会校验 JSON 格式。</span>
          </div>
          <button disabled={busy !== "" || !settings.length} onClick={() => void saveSettings()}>
            {busy === "settings" ? "保存中…" : "保存设置"}
          </button>
        </div>
        <div className="settings-list">
          {settings.map((setting, index) => (
            <label className="settings-editor" key={setting.key}>
              <span>{setting.key} {"·"} 更新于 {formatTime(setting.updatedAt)}</span>
              <textarea
                value={setting.valueJson}
                onChange={(event) => {
                  const next = [...settings];
                  next[index] = { ...setting, valueJson: event.target.value };
                  setSettings(next);
                }}
              />
            </label>
          ))}
          {!settings.length ? <EmptyState title="正在读取设置" description="默认设置会在首次启动时自动初始化。" /> : null}
        </div>
      </article>
      {message ? <div className="note">{message}</div> : null}
    </section>
  );
}

export const Settings = memo(SettingsBase);
