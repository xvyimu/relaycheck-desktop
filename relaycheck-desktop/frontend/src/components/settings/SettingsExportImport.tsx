import type { ExportResult } from "@/types";
import { formatBytes, formatTime } from "@/lib/format";
import { Button } from "@/components/ui/button";

export type SettingsExportImportProps = {
  exportPassword: string;
  importPassword: string;
  importFileName: string;
  exporting: boolean;
  importing: boolean;
  exportResult: ExportResult | null;
  exports: ExportResult[];
  onExportPasswordChange: (value: string) => void;
  onImportPasswordChange: (value: string) => void;
  onImportFileNameChange: (value: string) => void;
  onExport: () => void;
  onImport: () => void;
};

export function SettingsExportImport({
  exportPassword,
  importPassword,
  importFileName,
  exporting,
  importing,
  exportResult,
  exports,
  onExportPasswordChange,
  onImportPasswordChange,
  onImportFileNameChange,
  onExport,
  onImport,
}: SettingsExportImportProps) {
  return (
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
            onChange={(e) => onExportPasswordChange(e.target.value)}
            placeholder="设置导出密码"
          />
        </label>
        <button type="button" disabled={exporting || exportPassword.length < 6} onClick={() => void onExport()}>
          {exporting ? "导出中…" : "加密导出"}
        </button>
      </div>
      {exportResult ? (
        <div className="detail-list spacing-top-sm">
          <div>
            <span>文件名</span>
            <strong>{exportResult.fileName}</strong>
          </div>
          <div>
            <span>大小</span>
            <strong>{formatBytes(exportResult.sizeBytes)}</strong>
          </div>
          <div>
            <span>数据库</span>
            <strong>{formatBytes(exportResult.manifest.databaseSize)}</strong>
          </div>
          <div>
            <span>设置数</span>
            <strong>{exportResult.manifest.settingCount}</strong>
          </div>
          <div>
            <span>导出时间</span>
            <strong>{formatTime(exportResult.manifest.exportedAt)}</strong>
          </div>
        </div>
      ) : null}
      {exports.length > 0 ? (
        <div className="detail-list spacing-top-md">
          <div className="section-heading">
            <strong>已有导出文件</strong>
          </div>
          {exports.map((exp) => (
            <div key={exp.fileName}>
              <span>{exp.fileName}</span>
              <strong>
                {formatBytes(exp.sizeBytes)}
                <Button
                  variant="ghost"
                  type="button"
                  className="settings-inline-button"
                  onClick={() => onImportFileNameChange(exp.fileName)}
                >
                  选择导入
                </Button>
              </strong>
            </div>
          ))}
        </div>
      ) : null}
      <div className="proxy-form-grid spacing-top-md">
        <label className="field">
          <span>导入密码</span>
          <input
            type="password"
            value={importPassword}
            onChange={(e) => onImportPasswordChange(e.target.value)}
            placeholder="输入导出时设置的密码"
          />
        </label>
        <label className="field">
          <span>导入文件</span>
          <input
            value={importFileName}
            onChange={(e) => onImportFileNameChange(e.target.value)}
            placeholder="export-XXXXXXXX-XXXXXX.rczip"
          />
        </label>
        <button
          type="button"
          className="danger"
          disabled={importing || importPassword.length < 6 || !importFileName}
          onClick={() => void onImport()}
        >
          {importing ? "导入中…" : "加密导入"}
        </button>
      </div>
      <div className="problem-hint detail-hint">
        导出文件使用 AES-256-GCM 加密，包含完整数据库和所有设置。导入会覆盖当前数据，请先备份。
      </div>
    </article>
  );
}
