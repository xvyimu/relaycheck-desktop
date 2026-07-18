import { useState } from "react";

import { accountActionUrl } from "@/api/accounts";
import { api } from "@/api/client";
import {
  browserSessionOpenKind,
  browserSessionRunningLabel,
  formatBrowserLoginOpenMessage,
  formatBrowserLoginSaveMessage,
  formatLoginStatusTestMessage,
  isBrowserLoginOpenSuccess,
  isBrowserLoginSaveSuccess,
  isLoginStatusValid,
  RELOGIN_STEPS,
  reloginStepIndex,
  shouldShowReloginSteps,
  type BrowserSessionOpenKind,
  type ReloginUiPhase,
} from "@/lib/accountActions";
import { formatBalanceValue, formatTime } from "@/lib/format";
import { apiKeyStatusLabel, loginStatusLabel, statusLabel } from "@/lib/labels";
import type { Account, BrowserLoginOpenResponse, BrowserLoginSaveResponse, LoginStatusTestResponse } from "@/types";
import { TwoFactorGuide } from "@/components/ui/TwoFactorGuide";
import { Button } from "@/components/ui/button";

export function AccountDetailContent({ account, onClose }: { account: Account; onClose: () => void }) {
  const identity = account.email || account.username || account.authType;
  const checkinState = account.lastCheckinStatus || "";
  const keyState = account.apiKeyFingerprint ? apiKeyStatusLabel(account.apiKeyStatus || "unchecked") : "未保存";
  const needsTwoFactor = account.loginStatus === "two_factor_required";
  const [reloginPhase, setReloginPhase] = useState<ReloginUiPhase>("idle");
  const [sessionOpenKind, setSessionOpenKind] = useState<BrowserSessionOpenKind>(null);
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const showReloginSteps = shouldShowReloginSteps(account.loginStatus, account.lastCheckinStatus, reloginPhase);
  const activeStep = reloginStepIndex(reloginPhase, account.loginStatus);
  const isBusy = busy !== "";

  async function openBrowserLogin() {
    if (isBusy) return;
    setBusy("open");
    setMessage("");
    try {
      const result = await api<BrowserLoginOpenResponse>(accountActionUrl(account.id, "open-browser-login"), {
        method: "POST",
        body: JSON.stringify({}),
      });
      if (isBrowserLoginOpenSuccess(result.status)) {
        setReloginPhase("browser_open");
        setSessionOpenKind(browserSessionOpenKind(result.status));
      }
      setMessage(formatBrowserLoginOpenMessage(result));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "打开网页登录失败");
    } finally {
      setBusy("");
    }
  }

  async function saveAuth() {
    if (isBusy) return;
    setBusy("save");
    setMessage("");
    try {
      const result = await api<BrowserLoginSaveResponse>(accountActionUrl(account.id, "finish-browser-login"), {
        method: "POST",
        body: JSON.stringify({}),
      });
      if (isBrowserLoginSaveSuccess(result.status)) {
        setReloginPhase("auth_saved");
        setSessionOpenKind(null);
      }
      setMessage(formatBrowserLoginSaveMessage(result));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存授权失败");
    } finally {
      setBusy("");
    }
  }

  async function testLogin() {
    if (isBusy) return;
    setBusy("test");
    setMessage("");
    try {
      const result = await api<LoginStatusTestResponse>(accountActionUrl(account.id, "test-login"), {
        method: "POST",
        body: JSON.stringify({}),
      });
      if (isLoginStatusValid(result.status)) {
        setReloginPhase("auth_saved");
      }
      setMessage(formatLoginStatusTestMessage(result));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "测试登录态失败");
    } finally {
      setBusy("");
    }
  }

  return (
    <>
      <div className="detail-header">
        <div>
          <span className="eyebrow">账号详情</span>
          <strong>{account.displayName}</strong>
          <p>{account.upstreamSiteName || "未记录站点"}</p>
        </div>
        <div className="detail-actions">
          <Button variant="ghost" type="button" onClick={onClose}>
            关闭
          </Button>
        </div>
      </div>

      {showReloginSteps ? (
        <div className="account-relogin-steps" aria-label="重登步骤">
          {RELOGIN_STEPS.map((label, index) => {
            const stateClass = index < activeStep ? "is-done" : index === activeStep ? "is-current" : "";
            return (
              <span key={label} className={`account-relogin-step ${stateClass}`.trim()}>
                <b aria-hidden="true">{index + 1}</b>
                {label}
              </span>
            );
          })}
        </div>
      ) : null}

      {reloginPhase === "browser_open" ? (
        <div className="account-session-chip" role="status" aria-live="polite">
          {browserSessionRunningLabel(sessionOpenKind)}
        </div>
      ) : null}

      <div className="toolbar detail-relogin-actions">
        <button type="button" disabled={isBusy} onClick={() => void openBrowserLogin()}>
          {busy === "open" ? "打开中…" : "网页登录"}
        </button>
        <button type="button" disabled={isBusy || reloginPhase === "idle"} onClick={() => void saveAuth()}>
          {busy === "save" ? "保存中…" : "保存授权"}
        </button>
        <Button variant="ghost" type="button" disabled={isBusy} onClick={() => void testLogin()}>
          {busy === "test" ? "测试中…" : "测试登录态"}
        </Button>
      </div>

      {message ? (
        <div className="note" role="status" aria-live="polite">
          {message}
        </div>
      ) : null}

      <div className="detail-grid">
        <section className="detail-card">
          <h3>运营状态</h3>
          <div className="detail-metrics">
            <div>
              <span>登录</span>
              <strong>{loginStatusLabel(account.loginStatus)}</strong>
            </div>
            <div>
              <span>签到</span>
              <strong>{statusLabel(checkinState)}</strong>
            </div>
            <div>
              <span>余额</span>
              <strong>
                {account.balance !== undefined
                  ? formatBalanceValue(account.balance, account.balanceUnit || "unknown")
                  : "-"}
              </strong>
            </div>
          </div>
          <div className="detail-list">
            <div>
              <span>标识</span>
              <strong>{identity}</strong>
            </div>
            <div>
              <span>认证</span>
              <strong>{account.authType}</strong>
            </div>
            <div>
              <span>最近签到</span>
              <strong>{formatTime(account.lastCheckinAt || "")}</strong>
            </div>
            <div>
              <span>验证时间</span>
              <strong>{formatTime(account.lastValidatedAt || "")}</strong>
            </div>
          </div>
        </section>

        <section className="detail-card">
          <h3>Key 与模型</h3>
          <div className="detail-list">
            <div>
              <span>指纹</span>
              <strong>{account.apiKeyFingerprint || "未保存"}</strong>
            </div>
            <div>
              <span>检测状态</span>
              <strong>{keyState}</strong>
            </div>
            <div>
              <span>测试模型</span>
              <strong>{account.apiKeyTestModel || "未测速"}</strong>
            </div>
            <div>
              <span>延迟</span>
              <strong>{account.apiKeyLatencyMs ? `${account.apiKeyLatencyMs}ms` : "未测速"}</strong>
            </div>
          </div>
          {account.apiKeySampleModels?.length ? (
            <div className="signal-list">
              {account.apiKeySampleModels.slice(0, 8).map((model) => (
                <span key={model}>{model}</span>
              ))}
            </div>
          ) : null}
          {account.apiKeyTestMessage ? (
            <div className="problem-hint detail-hint">{account.apiKeyTestMessage}</div>
          ) : null}
        </section>

        <section className="detail-card">
          <h3>建议动作</h3>
          <div className="detail-stack">
            {needsTwoFactor ? (
              <TwoFactorGuide
                variant="inline"
                siteName={account.upstreamSiteName}
                baseUrl={account.upstreamSiteBaseUrl}
                loginUrl={account.upstreamSiteLoginUrl}
              />
            ) : null}
            {account.loginStatus !== "valid" && !needsTwoFactor ? (
              <div className="problem-hint detail-hint">
                登录态异常，需重新登录或保存授权。不自动填密码、不绕过 2FA。
              </div>
            ) : null}
            {!["success", "already_checked"].includes(checkinState) ? (
              <div className="problem-hint detail-hint">最近签到未确认成功，建议在签到页查看返回消息。</div>
            ) : null}
            {account.apiKeyFingerprint && account.apiKeyStatus !== "valid" ? (
              <div className="problem-hint detail-hint">API Key 状态异常，需要重新检测。</div>
            ) : null}
            {account.balance === undefined ? (
              <div className="problem-hint detail-hint">暂无余额快照，刷新余额后再做趋势判断。</div>
            ) : null}
            {account.loginStatus === "valid" &&
            ["success", "already_checked"].includes(checkinState) &&
            (!account.apiKeyFingerprint || account.apiKeyStatus === "valid") ? (
              <div className="note">账号状态正常，已是最佳状态。</div>
            ) : null}
          </div>
        </section>
      </div>
    </>
  );
}
