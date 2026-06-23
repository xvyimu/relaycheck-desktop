import { useState } from "react";
import { api } from "@/api/client";
import type { ChromePasswordPreview } from "@/types";

interface ImportDialogProps {
  onDone: () => void;
}

export function LegacyConfigImport({ onDone }: ImportDialogProps) {
  const [configContent, setConfigContent] = useState("");
  const [fileName, setFileName] = useState("");
  const [message, setMessage] = useState("");

  async function loadFile(file?: File) {
    setMessage("");
    if (!file) return;
    if (file.size > 1024 * 1024) {
      setMessage("配置文件太大，请选择单个 config_site*.json。");
      return;
    }
    setFileName(file.name);
    setConfigContent(await file.text());
  }

  return (
    <div className="card">
      <div>
        <strong>旧版 config_site*.json 导入</strong>
        <p>兼容旧 Python 工具的 login_url/checkin_url/balance_url。导入后会写入站点自定义签到规则；有账号密码时才创建账号。</p>
      </div>
      <div className="toolbar">
        <input type="file" accept=".json,application/json" onChange={(event) => void loadFile(event.target.files?.[0])} />
        <button
          disabled={!configContent}
          onClick={async () => {
            const result = await api<{
              baseUrl: string;
              siteCreated: boolean;
              accountImported: boolean;
              hasCheckinRule: boolean;
              hasBalanceRule: boolean;
            }>("/api/accounts/import-legacy-config", {
              method: "POST",
              body: JSON.stringify({ configContent, fileName }),
            });
            setMessage(
              `已导入 ${result.baseUrl}，站点${result.siteCreated ? "已创建" : "已合并"}，账号${result.accountImported ? "已导入" : "未新增"}，签到规则${result.hasCheckinRule ? "已写入" : "无"}。`,
            );
            await onDone();
          }}
        >
          导入旧配置
        </button>
        {fileName ? <span className="note inline-note">{fileName}</span> : null}
      </div>
      <textarea
        value={configContent}
        onChange={(event) => setConfigContent(event.target.value)}
        placeholder='也可以粘贴 {"login_url":"https://.../api/user/login","checkin_url":"https://.../api/user/checkin"}'
      />
      {message ? <div className="note">{message}</div> : null}
    </div>
  );
}

export function ChromePasswordImport({ onDone }: ImportDialogProps) {
  const [csvContent, setCsvContent] = useState("");
  const [fileName, setFileName] = useState("");
  const [preview, setPreview] = useState<ChromePasswordPreview | null>(null);
  const [message, setMessage] = useState("");

  async function loadFile(file?: File) {
    setMessage("");
    setPreview(null);
    if (!file) return;
    if (file.size > 8 * 1024 * 1024) {
      setMessage("CSV 文件太大，请分批导入。");
      return;
    }
    setFileName(file.name);
    setCsvContent(await file.text());
  }

  return (
    <div className="card">
      <div>
        <strong>Chrome/Via 密码 CSV 匹配导入</strong>
        <p>请在浏览器密码管理器中手动导出 CSV 后选择文件。工具不会读取或解密浏览器内部密码库。</p>
      </div>
      <div className="toolbar">
        <input
          type="file"
          accept=".csv,text/csv"
          onChange={(event) => void loadFile(event.target.files?.[0])}
        />
        <button
          disabled={!csvContent}
          onClick={async () => {
            const result = await api<ChromePasswordPreview>("/api/accounts/import-chrome-passwords/preview", {
              method: "POST",
              body: JSON.stringify({ csvContent }),
            });
            setPreview(result);
            setMessage(`已读取 ${result.totalRows} 条，匹配 ${result.matchedRows} 条，覆盖 ${result.uniqueSiteCount} 个站点。`);
          }}
        >
          预览匹配
        </button>
        <button
          disabled={!csvContent || !preview?.matchedRows}
          onClick={async () => {
            const result = await api<{ importedCount: number; matchedRows: number; skippedExisting: number }>(
              "/api/accounts/import-chrome-passwords/import",
              {
                method: "POST",
                body: JSON.stringify({ csvContent }),
              },
            );
            setMessage(`已导入 ${result.importedCount} 个账号，匹配 ${result.matchedRows} 条，跳过 ${result.skippedExisting} 个已存在账号。`);
            await onDone();
          }}
        >
          确认导入匹配账号
        </button>
        {fileName ? <span className="note inline-note">{fileName}</span> : null}
      </div>
      {message ? <div className="note">{message}</div> : null}
      {preview ? (
        <div className="list compact">
          {preview.matches.slice(0, 12).map((match) => (
            <article className="item wide" key={`${match.siteId}-${match.url}-${match.username}`}>
              <div>
                <strong>{match.siteName} · {match.username}</strong>
                <span>{match.url} · 密码 {match.passwordMasked} · {match.existingAccount ? "已存在，将跳过" : "可导入"}</span>
              </div>
            </article>
          ))}
          {preview.matches.length > 12 ? <div className="note">仅预览前 12 条，确认导入会处理全部匹配项。</div> : null}
        </div>
      ) : null}
    </div>
  );
}