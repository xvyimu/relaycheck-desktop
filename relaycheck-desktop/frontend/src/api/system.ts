import { api } from "@/api/client";
import type {
  AuditLogItem,
  ExportResult,
  PortCheckResult,
  ProxyTestResult,
  SchedulerStatus,
  SystemBackup,
  SystemSetting,
  VersionCheckResult,
} from "@/types";

export type SystemSettingWriteItem = {
  key: string;
  valueJson: string;
};

export type PersistSettingsResult = {
  updated: number;
};

export type RestoreBackupResult = {
  restored: boolean;
  fileName: string;
  beforeBackup: SystemBackup;
};

export type DeleteBackupsResult = {
  deleted: number;
  skipped: string[];
};

/** 读取系统设置列表。 */
function listSettings(): Promise<SystemSetting[]> {
  return api<SystemSetting[]>("/api/system/settings");
}

/** 批量保存设置；body 仅含 settings 数组。 */
function saveSettings(settings: SystemSettingWriteItem[]): Promise<PersistSettingsResult> {
  return api<PersistSettingsResult>("/api/system/settings", {
    method: "PUT",
    body: JSON.stringify({ settings }),
  });
}

/** 列出本地备份文件元数据。 */
function listBackups(): Promise<SystemBackup[]> {
  return api<SystemBackup[]>("/api/system/backups");
}

/** 创建当前数据库备份。 */
function createBackup(): Promise<SystemBackup> {
  return api<SystemBackup>("/api/system/backup", { method: "POST" });
}

/** 从指定备份恢复；仅发送 fileName。 */
function restoreBackup(fileName: string): Promise<RestoreBackupResult> {
  return api<RestoreBackupResult>("/api/system/restore", {
    method: "POST",
    body: JSON.stringify({ fileName }),
  });
}

/** 删除选中备份；仅发送 fileNames。 */
function deleteBackups(fileNames: string[]): Promise<DeleteBackupsResult> {
  return api<DeleteBackupsResult>("/api/system/backups/delete", {
    method: "POST",
    body: JSON.stringify({ fileNames }),
  });
}

/** 读取调度器状态。 */
function schedulerStatus(): Promise<SchedulerStatus> {
  return api<SchedulerStatus>("/api/system/scheduler-status");
}

/** 读取审计日志列表。 */
function auditLog(): Promise<AuditLogItem[]> {
  return api<AuditLogItem[]>("/api/system/audit-log");
}

/** 列出加密导出产物；失败由调用方降级为空数组。 */
function listExports(): Promise<ExportResult[]> {
  return api<ExportResult[]>("/api/system/exports");
}

/** 测试网络代理；仅发送 targetUrl。 */
function proxyTest(targetUrl: string): Promise<ProxyTestResult> {
  return api<ProxyTestResult>("/api/system/proxy-test", {
    method: "POST",
    body: JSON.stringify({ targetUrl }),
  });
}

/** 检查更新；只读 GET。 */
function versionCheck(): Promise<VersionCheckResult> {
  return api<VersionCheckResult>("/api/system/version-check");
}

/** 端口占用探测；port 走 query 且 encode。 */
function portCheck(port: string | number): Promise<PortCheckResult> {
  return api<PortCheckResult>(`/api/system/port-check?port=${encodeURIComponent(String(port))}`);
}

/** 加密导出数据库；仅发送 password。 */
function exportDatabase(password: string): Promise<ExportResult> {
  return api<ExportResult>("/api/system/export", {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

/** 加密导入数据库；仅发送 password 与 fileName。 */
function importDatabase(password: string, fileName: string): Promise<unknown> {
  return api("/api/system/import", {
    method: "POST",
    body: JSON.stringify({ password, fileName }),
  });
}

export const systemApi = {
  listSettings,
  saveSettings,
  listBackups,
  createBackup,
  restoreBackup,
  deleteBackups,
  schedulerStatus,
  auditLog,
  listExports,
  proxyTest,
  versionCheck,
  portCheck,
  exportDatabase,
  importDatabase,
} as const;
