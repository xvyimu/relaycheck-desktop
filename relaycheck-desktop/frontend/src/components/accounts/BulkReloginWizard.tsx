import { useMemo, useState } from "react";

import { api } from "@/api/client";
import { isProblemAccount } from "@/components/accounts/helpers";
import { RELOGIN_STEPS } from "@/lib/accountActions";
import type {
  Account,
  BulkBrowserOpenResponse,
  BulkBrowserSaveResponse,
} from "@/types";

export type BulkReloginPhase = "idle" | "opened" | "saved" | "done";

export type BulkReloginWizardProps = {
  accounts: Account[];
  onDone: () => void | Promise<void>;
};

export function bulkReloginCandidates(accounts: Account[]): Account[] {
  return accounts.filter(isProblemAccount);
}

export function BulkReloginWizard({ accounts, onDone }: BulkReloginWizardProps) {
  const candidates = useMemo(() => bulkReloginCandidates(accounts), [accounts]);
  const [open, setOpen] = useState(false);
  const [phase, setPhase] = useState<BulkReloginPhase>("idle");
  const [limit, setLimit] = useState(5);
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [lastOpen, setLastOpen] = useState<BulkBrowserOpenResponse | null>(null);
  const [lastSave, setLastSave] = useState<BulkBrowserSaveResponse | null>(null);

  const stepIndex = phase === "idle" ? 0 : phase === "opened" ? 1 : phase === "saved" ? 2 : 3;

  async function openBatch() {
    if (busy) return;
    setBusy("open");
    setMessage("");
    try {
      const result = await api<BulkBrowserOpenResponse>("/api/accounts/bulk-open-browser-login", {
        method: "POST",
        body: JSON.stringify({ limit }),
      });
      setLastOpen(result);
      setPhase("opened");
      setMessage(
        `已打开/复用 ${result.opened} 个登录窗口，失败 ${result.failed} 个。请在浏览器中完成登录（含 2FA/验证码），不要关闭窗口，然后点「批量保存授权」。`,
      );
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "批量打开网页登录失败");
    } finally {
      setBusy("");
    }
  }

  async function saveBatch() {
    if (busy) return;
    setBusy("save");
    setMessage("");
    try {
      const result = await api<BulkBrowserSaveResponse>("/api/accounts/bulk-finish-browser-login", {
        method: "POST",
        body: JSON.stringify({}),
      });
      setLastSave(result);
      setPhase("saved");
      setMessage(
        `保存授权 ${result.saved} 个，失败/未完成 ${result.failed} 个。可在账号卡上「测试登录态」或直接签到。`,
      );
      await onDone();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "批量保存授权失败");
    } finally {
      setBusy("");
    }
  }

  function resetWizard() {
    setPhase("idle");
    setMessage("");
    setLastOpen(null);
    setLastSave(null);
  }

  if (!candidates.length && !open) {
    return null;
  }

  return (
    <div className="bulk-relogin-wizard card" data-testid="bulk-relogin-wizard">
      <div className="bulk-relogin-head">
        <div>
          <strong>批量会话重登</strong>
          <span>
            异常账号 {candidates.length} 个。人工完成浏览器登录后保存授权；不自动填密码、不绕过 2FA。
          </span>
        </div>
        <button
          type="button"
          className="ghost"
          aria-expanded={open}
          onClick={() => setOpen((current) => !current)}
        >
          {open ? "收起向导" : "打开向导"}
        </button>
      </div>

      {open ? (
        <>
          <div className="account-relogin-steps bulk-relogin-steps" aria-label="批量重登步骤">
            {RELOGIN_STEPS.map((label, index) => {
              const stateClass =
                index < stepIndex ? "is-done" : index === stepIndex ? "is-current" : "";
              return (
                <span key={label} className={`account-relogin-step ${stateClass}`.trim()}>
                  <b aria-hidden="true">{index + 1}</b>
                  {label}
                </span>
              );
            })}
          </div>

          <ol className="bulk-relogin-guide">
            <li>点「批量打开」为异常账号打开 Chrome 登录页（可限制数量）。</li>
            <li>在浏览器内完成登录；若有 2FA/验证码请在页面内完成。</li>
            <li>回到本工具点「批量保存授权」写入加密会话。</li>
            <li>在账号卡上测试登录态或签到；失败账号可单卡重试。</li>
          </ol>

          <div className="bulk-relogin-controls">
            <label className="field">
              <span>本批打开上限</span>
              <input
                type="number"
                min={1}
                max={20}
                value={limit}
                onChange={(event) => setLimit(Math.max(1, Math.min(20, Number(event.target.value) || 1)))}
                disabled={Boolean(busy)}
              />
            </label>
            <div className="toolbar">
              <button type="button" disabled={Boolean(busy) || !candidates.length} onClick={() => void openBatch()}>
                {busy === "open" ? "打开中…" : "1. 批量打开"}
              </button>
              <button
                type="button"
                disabled={Boolean(busy) || phase === "idle"}
                onClick={() => void saveBatch()}
              >
                {busy === "save" ? "保存中…" : "2. 批量保存授权"}
              </button>
              <button type="button" className="ghost" disabled={Boolean(busy)} onClick={resetWizard}>
                重置步骤
              </button>
            </div>
          </div>

          {lastOpen ? (
            <div className="bulk-relogin-stats">
              <span>上次打开 处理 {lastOpen.processed}</span>
              <span>成功 {lastOpen.opened}</span>
              <span>失败 {lastOpen.failed}</span>
            </div>
          ) : null}
          {lastSave ? (
            <div className="bulk-relogin-stats">
              <span>上次保存 处理 {lastSave.processed}</span>
              <span>成功 {lastSave.saved}</span>
              <span>失败 {lastSave.failed}</span>
            </div>
          ) : null}

          {message ? (
            <div className="note" role="status" aria-live="polite">
              {message}
            </div>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
