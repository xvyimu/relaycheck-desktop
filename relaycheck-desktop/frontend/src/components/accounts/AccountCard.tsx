import { useEffect, useState } from "react";
import { accountApi } from "@/api/accounts";
import {
  accountActionButtonLabel,
  appendReloginHint,
  browserSessionOpenKind,
  browserSessionRunningLabel,
  formatBrowserLoginOpenMessage,
  formatBrowserLoginSaveMessage,
  formatLoginStatusTestMessage,
  isBrowserLoginOpenSuccess,
  isBrowserLoginSaveSuccess,
  isLikelyAuthFailureMessage,
  isLoginStatusValid,
  primaryActionsForRelogin,
  RELOGIN_STEPS,
  reloginStepIndex,
  shouldShowReloginSteps,
  type BrowserSessionOpenKind,
  type PrimaryActionKey,
  type ReloginUiPhase,
} from "@/lib/accountActions";
import { formatBalanceValue, formatTime } from "@/lib/format";
import {
  apiKeyStatusLabel,
  formatAPIKeyTestMessage,
  loginStatusLabel,
  statusLabel,
  upstreamKindLabel,
} from "@/lib/labels";
import type {
  Account,
  APIKeyTestResult,
  BrowserLoginOpenResponse,
  BrowserLoginSaveResponse,
  LoginStatusTestResponse,
} from "@/types";
import { AccountKeySummary } from "@/components/accounts/AccountKeySummary";
import { AccountCardEditor } from "@/components/accounts/AccountCardEditor";
import {
  accountAvatarLabel,
  accountBackendShort,
  accountDomainLabel,
  defaultLoginUrl,
  isProblemAccount,
} from "@/components/accounts/helpers";
import { StatusLabel } from "@/components/ui/status-label";
import { TwoFactorGuide } from "@/components/ui/TwoFactorGuide";
import { Button } from "@/components/ui/button";

interface AccountCardProps {
  account: Account;
  onDone: () => void;
  onOpenDetail: () => void;
}

export function AccountCard({ account, onDone, onOpenDetail }: AccountCardProps) {
  const [editing, setEditing] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const [showTwoFactorGuide, setShowTwoFactorGuide] = useState(false);
  const [dismissedTwoFactor, setDismissedTwoFactor] = useState(false);
  const [reloginPhase, setReloginPhase] = useState<ReloginUiPhase>("idle");
  const [sessionOpenKind, setSessionOpenKind] = useState<BrowserSessionOpenKind>(null);
  const [displayName, setDisplayName] = useState(account.displayName);
  const [siteName, setSiteName] = useState(account.upstreamSiteName);
  const [baseUrl, setBaseUrl] = useState(account.upstreamSiteBaseUrl || "");
  const [loginUrl, setLoginUrl] = useState(
    account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || ""),
  );
  const [kind, setKind] = useState(account.upstreamSiteKind || "auto");
  const [siteUpdateScope, setSiteUpdateScope] = useState<"current" | "shared">("current");
  const [email, setEmail] = useState(account.email || "");
  const [username, setUsername] = useState(account.username || "");
  const [authType, setAuthType] = useState(account.authType);
  const [password, setPassword] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [clearApiKey, setClearApiKey] = useState(false);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState("");
  const isProblem = isProblemAccount(account);
  const isBusy = busy !== "";
  const isMessageError =
    message.includes("失败") ||
    message.includes("错误") ||
    message.includes("失效") ||
    isLikelyAuthFailureMessage(message);
  const primaryKeys = primaryActionsForRelogin(reloginPhase);
  const showReloginSteps = shouldShowReloginSteps(account.loginStatus, account.lastCheckinStatus, reloginPhase);
  const activeStep = reloginStepIndex(reloginPhase, account.loginStatus);
  const saveIsPrimary = primaryKeys.includes("save");
  const testIsPrimary = primaryKeys.includes("test");

  useEffect(() => {
    setDisplayName(account.displayName);
    setSiteName(account.upstreamSiteName);
    setBaseUrl(account.upstreamSiteBaseUrl || "");
    setLoginUrl(account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || ""));
    setKind(account.upstreamSiteKind || "auto");
    setEmail(account.email || "");
    setUsername(account.username || "");
    setAuthType(account.authType);
    setPassword("");
    setApiKey("");
    setClearApiKey(false);
    setSiteUpdateScope("current");
    setDismissedTwoFactor(false);
  }, [
    account.id,
    account.displayName,
    account.upstreamSiteName,
    account.upstreamSiteBaseUrl,
    account.upstreamSiteLoginUrl,
    account.upstreamSiteKind,
    account.email,
    account.username,
    account.authType,
  ]);

  useEffect(() => {
    setReloginPhase("idle");
    setSessionOpenKind(null);
    setMessage("");
    setMoreOpen(false);
  }, [account.id]);

  async function runAction<T>(label: string, action: () => Promise<T>, formatSuccess?: (result: T) => string) {
    if (isBusy) return;
    setBusy(label);
    setMessage("");
    try {
      const result = await action();
      await onDone();
      setMessage(formatSuccess ? formatSuccess(result) : `${label}完成。`);
    } catch (error) {
      const raw = error instanceof Error ? error.message : `${label}失败`;
      const withHint =
        (label === "签到" || label === "刷新余额" || label === "测试登录态") && isLikelyAuthFailureMessage(raw)
          ? appendReloginHint(raw)
          : raw;
      setMessage(withHint);
    } finally {
      setBusy("");
    }
  }

  async function openBrowserLogin() {
    await runAction(
      "网页登录",
      async () => {
        const result = await accountApi.postAction<BrowserLoginOpenResponse>(account.id, "open-browser-login");
        if (isBrowserLoginOpenSuccess(result.status)) {
          setReloginPhase("browser_open");
          setSessionOpenKind(browserSessionOpenKind(result.status));
        }
        return result;
      },
      formatBrowserLoginOpenMessage,
    );
  }

  async function finishBrowserLogin() {
    await runAction(
      "保存授权",
      async () => {
        const result = await accountApi.postAction<BrowserLoginSaveResponse>(account.id, "finish-browser-login");
        if (isBrowserLoginSaveSuccess(result.status)) {
          setReloginPhase("auth_saved");
          setSessionOpenKind(null);
        }
        return result;
      },
      formatBrowserLoginSaveMessage,
    );
  }

  async function testLoginStatus() {
    await runAction(
      "测试登录态",
      async () => {
        const result = await accountApi.postAction<LoginStatusTestResponse>(account.id, "test-login");
        if (isLoginStatusValid(result.status)) {
          setReloginPhase("idle");
        }
        return result;
      },
      formatLoginStatusTestMessage,
    );
  }

  async function saveAccount() {
    if (clearApiKey) {
      const confirmed = window.confirm(
        `确认清空"${account.displayName}"当前保存的 API Key？保存后需要重新录入密钥才能恢复模型检测。`,
      );
      if (!confirmed) return;
    }
    await runAction("保存账号", async () => {
      await accountApi.update(account.id, {
        displayName,
        siteName,
        baseUrl,
        loginUrl,
        kind: kind === "auto" ? "" : kind,
        email,
        username,
        authType,
        password,
        apiKey,
        clearApiKey,
        siteUpdateScope,
      });
      setEditing(false);
    });
  }

  async function testAPIKey() {
    if (isBusy) return;
    setBusy("检测密钥");
    setMessage("");
    try {
      const result = await accountApi.postAction<APIKeyTestResult>(account.id, "test-api-key");
      await onDone();
      setMessage(formatAPIKeyTestMessage(result));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "检测密钥失败");
    } finally {
      setBusy("");
    }
  }

  async function deleteAccount() {
    const confirmed = window.confirm(
      `确认删除账号"${account.displayName}"？这会删除该账号保存的密码、Cookie、Token 和 API Key 等凭据。`,
    );
    if (!confirmed) return;
    await runAction("删除账号", () => accountApi.remove(account.id));
  }

  function renderPrimaryButton(key: PrimaryActionKey) {
    switch (key) {
      case "open":
        return (
          <button
            key="open"
            type="button"
            disabled={isBusy}
            aria-label={`打开 ${account.displayName} 的网页登录`}
            onClick={() => void openBrowserLogin()}
          >
            {accountActionButtonLabel("网页登录", busy)}
          </button>
        );
      case "save":
        return (
          <button
            key="save"
            type="button"
            disabled={isBusy}
            aria-label={`保存 ${account.displayName} 的浏览器授权`}
            onClick={() => void finishBrowserLogin()}
          >
            {accountActionButtonLabel("保存授权", busy)}
          </button>
        );
      case "test":
        return (
          <button
            key="test"
            type="button"
            disabled={isBusy}
            aria-label={`测试 ${account.displayName} 的登录态`}
            onClick={() => void testLoginStatus()}
          >
            {accountActionButtonLabel("测试登录态", busy, "检测中…")}
          </button>
        );
      case "checkin":
        return (
          <button
            key="checkin"
            type="button"
            disabled={isBusy}
            aria-label={`为 ${account.displayName} 执行签到`}
            onClick={() =>
              void runAction("签到", () => accountApi.postAction(account.id, "checkin"))
            }
          >
            {accountActionButtonLabel("签到", busy)}
          </button>
        );
      case "detail":
        return (
          <Button variant="ghost" key="detail" type="button" disabled={isBusy} onClick={onOpenDetail}>
            详情
          </Button>
        );
      default:
        return null;
    }
  }

  return (
    <article className={`account-card account-card-v4 ${isProblem ? "is-problem" : ""}`} aria-busy={isBusy}>
      <div className="account-card-head">
        <div
          className="account-avatar-stack"
          aria-label={`${accountDomainLabel(account)}，${upstreamKindLabel(account.upstreamSiteKind || "unknown")}`}
        >
          <div className="account-avatar" aria-hidden="true">
            {accountAvatarLabel(account)}
          </div>
          <span className={`account-kind-chip kind-${account.upstreamSiteKind || "unknown"}`}>
            {accountBackendShort(account.upstreamSiteKind || "unknown")}
          </span>
        </div>
        <div className="account-identity">
          <span title={account.upstreamSiteName}>{account.upstreamSiteName}</span>
          <strong title={account.displayName}>{account.displayName}</strong>
          <em title={account.upstreamSiteBaseUrl || "未记录站点地址"}>
            {account.upstreamSiteBaseUrl || "未记录站点地址"}
          </em>
        </div>
        <div className={`account-status status-${account.loginStatus}`}>
          <StatusLabel level={account.loginStatus} label={loginStatusLabel(account.loginStatus)} />
        </div>
      </div>

      <div className="account-card-metrics">
        <div className="metric-account">
          <span>账号</span>
          <strong>{account.email || account.username || account.authType}</strong>
        </div>
        <div className="metric-checkin">
          <span>签到</span>
          <strong>{statusLabel(account.lastCheckinStatus || "")}</strong>
        </div>
        <div className="metric-balance">
          <span>余额</span>
          <strong>
            {account.balance !== undefined
              ? formatBalanceValue(account.balance, account.balanceUnit || "unknown")
              : "-"}
          </strong>
        </div>
        <div className="metric-key">
          <span>Key</span>
          <strong>
            {account.apiKeyFingerprint ? apiKeyStatusLabel(account.apiKeyStatus || "unchecked") : "未保存"}
          </strong>
        </div>
      </div>

      <div className="chips secondary-chips">
        <span>{account.authType}</span>
        {account.apiKeyFingerprint ? (
          <span>
            {account.apiKeyFingerprint} · {apiKeyStatusLabel(account.apiKeyStatus || "unchecked")}
          </span>
        ) : (
          <span>未保存密钥</span>
        )}
        {account.lastCheckinAt ? <span>签到 {formatTime(account.lastCheckinAt)}</span> : null}
      </div>

      {account.apiKeyFingerprint ? <AccountKeySummary account={account} /> : null}

      {account.lastCheckinMessage ? <div className="problem-hint">{account.lastCheckinMessage}</div> : null}

      {showReloginSteps ? (
        <div className="account-relogin-steps" aria-label="会话重登步骤">
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

      {account.loginStatus === "two_factor_required" && !dismissedTwoFactor ? (
        <TwoFactorGuide
          variant="inline"
          siteName={account.upstreamSiteName}
          baseUrl={account.upstreamSiteBaseUrl}
          loginUrl={account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || "")}
          onClose={() => setDismissedTwoFactor(true)}
          onOpenBrowserLogin={() => void openBrowserLogin()}
        />
      ) : null}

      {editing ? (
        <AccountCardEditor
          siteName={siteName}
          setSiteName={setSiteName}
          kind={kind}
          setKind={setKind}
          baseUrl={baseUrl}
          setBaseUrl={setBaseUrl}
          loginUrl={loginUrl}
          setLoginUrl={setLoginUrl}
          siteUpdateScope={siteUpdateScope}
          setSiteUpdateScope={setSiteUpdateScope}
          displayName={displayName}
          setDisplayName={setDisplayName}
          email={email}
          setEmail={setEmail}
          username={username}
          setUsername={setUsername}
          authType={authType}
          setAuthType={setAuthType}
          password={password}
          setPassword={setPassword}
          apiKey={apiKey}
          setApiKey={setApiKey}
          hasAPIKey={Boolean(account.apiKeyFingerprint)}
          clearApiKey={clearApiKey}
          setClearApiKey={setClearApiKey}
          busy={busy}
          isBusy={isBusy}
          onSave={saveAccount}
          onCancel={() => setEditing(false)}
        />
      ) : null}

      <div className="account-card-actions">
        <div className="account-action-group primary">
          {primaryKeys.map((key) => renderPrimaryButton(key))}
          <Button
            variant="ghost"
            type="button"
            className={`more-toggle ${moreOpen ? "active" : ""}`}
            disabled={isBusy}
            aria-expanded={moreOpen}
            onClick={() => setMoreOpen((current) => !current)}
          >
            {moreOpen ? "收起" : "更多"}
          </Button>
        </div>
        {moreOpen ? (
          <div className="account-more-panel">
            <div className="account-action-label">会话与余额</div>
            <div className="account-action-group secondary">
              {!saveIsPrimary ? (
                <Button
                  variant="ghost"
                  type="button"
                  disabled={isBusy}
                  aria-label={`保存 ${account.displayName} 的浏览器授权`}
                  onClick={() => void finishBrowserLogin()}
                >
                  {accountActionButtonLabel("保存授权", busy)}
                </Button>
              ) : null}
              {!testIsPrimary ? (
                <Button
                  variant="ghost"
                  type="button"
                  disabled={isBusy}
                  aria-label={`测试 ${account.displayName} 的登录态`}
                  onClick={() => void testLoginStatus()}
                >
                  {accountActionButtonLabel("测试登录态", busy, "检测中…")}
                </Button>
              ) : null}
              {!primaryKeys.includes("checkin") ? (
                <Button
                  variant="ghost"
                  type="button"
                  disabled={isBusy}
                  aria-label={`为 ${account.displayName} 执行签到`}
                  onClick={() =>
                    void runAction("签到", () => accountApi.postAction(account.id, "checkin"))
                  }
                >
                  {accountActionButtonLabel("签到", busy)}
                </Button>
              ) : null}
              <Button
                variant="ghost"
                type="button"
                disabled={isBusy}
                aria-label={`刷新 ${account.displayName} 的余额`}
                onClick={() =>
                  void runAction("刷新余额", () => accountApi.postAction(account.id, "refresh-balance"))
                }
              >
                {accountActionButtonLabel("刷新余额", busy)}
              </Button>
            </div>
            <div className="account-action-label">维护操作</div>
            <div className="account-action-group secondary">
              <Button variant="ghost" type="button" disabled={isBusy} onClick={() => setEditing((current) => !current)}>
                {editing ? "收起编辑" : "编辑账号"}
              </Button>
              <Button
                variant="ghost"
                type="button"
                disabled={!account.apiKeyFingerprint || isBusy}
                onClick={() => void testAPIKey()}
              >
                {accountActionButtonLabel("检测密钥", busy, "检测中…")}
              </Button>
              <Button variant="ghost" type="button" disabled={isBusy} onClick={() => setShowTwoFactorGuide(true)}>
                2FA 指引
              </Button>
            </div>
            <div className="account-action-label danger-label">危险操作</div>
            <div className="account-action-group danger-zone">
              <button type="button" className="danger" disabled={isBusy} onClick={() => void deleteAccount()}>
                {accountActionButtonLabel("删除账号", busy)}
              </button>
            </div>
          </div>
        ) : null}
      </div>
      {message ? (
        <div
          className={isMessageError ? "error" : "note"}
          role={isMessageError ? "alert" : "status"}
          aria-live={isMessageError ? "assertive" : "polite"}
        >
          {message}
        </div>
      ) : null}

      {showTwoFactorGuide ? (
        <TwoFactorGuide
          variant="dialog"
          siteName={account.upstreamSiteName}
          baseUrl={account.upstreamSiteBaseUrl}
          loginUrl={account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || "")}
          onClose={() => setShowTwoFactorGuide(false)}
          onOpenBrowserLogin={() => {
            setShowTwoFactorGuide(false);
            void openBrowserLogin();
          }}
        />
      ) : null}
    </article>
  );
}
