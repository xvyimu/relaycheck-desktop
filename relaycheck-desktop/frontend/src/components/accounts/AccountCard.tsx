import { useEffect, useState } from "react";
import { api } from "@/api/client";
import { formatBalanceValue, formatTime } from "@/lib/format";
import { apiKeyStatusLabel, formatAPIKeyTestMessage, loginStatusLabel, statusLabel, upstreamKindLabel } from "@/lib/labels";
import type { Account, APIKeyTestResult } from "@/types";
import { AccountKeySummary } from "@/components/accounts/AccountKeySummary";
import { accountAvatarLabel, accountBackendShort, accountDomainLabel, defaultLoginUrl, isProblemAccount } from "@/components/accounts/helpers";
import { StatusLabel } from "@/components/ui/status-label";
import { TwoFactorGuide } from "@/components/ui/TwoFactorGuide";

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
  const [displayName, setDisplayName] = useState(account.displayName);
  const [siteName, setSiteName] = useState(account.upstreamSiteName);
  const [baseUrl, setBaseUrl] = useState(account.upstreamSiteBaseUrl || "");
  const [loginUrl, setLoginUrl] = useState(account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || ""));
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
  }, [account.id, account.displayName, account.upstreamSiteName, account.upstreamSiteBaseUrl, account.upstreamSiteLoginUrl, account.upstreamSiteKind, account.email, account.username, account.authType]);

  async function runAction(label: string, action: () => Promise<unknown>) {
    if (busy) return;
    setBusy(label);
    setMessage("");
    try {
      await action();
      await onDone();
      setMessage(`${label}完成。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `${label}失败`);
    } finally {
      setBusy("");
    }
  }

  async function saveAccount() {
    if (clearApiKey) {
      const confirmed = window.confirm(`确认清空"${account.displayName}"当前保存的 API Key？保存后需要重新录入密钥才能恢复模型检测。`);
      if (!confirmed) return;
    }
    await runAction("保存账号", async () => {
      await api(`/api/accounts/${account.id}`, {
        method: "PUT",
        body: JSON.stringify({
          displayName, siteName, baseUrl, loginUrl,
          kind: kind === "auto" ? "" : kind,
          email, username, authType, password, apiKey, clearApiKey, siteUpdateScope,
        }),
      });
      setEditing(false);
    });
  }

  async function testAPIKey() {
    if (busy) return;
    setBusy("检测密钥");
    setMessage("");
    try {
      const result = await api<APIKeyTestResult>(`/api/accounts/${account.id}/test-api-key`, { method: "POST" });
      await onDone();
      setMessage(formatAPIKeyTestMessage(result));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "检测密钥失败");
    } finally {
      setBusy("");
    }
  }

  async function deleteAccount() {
    const confirmed = window.confirm(`确认删除账号"${account.displayName}"？这会删除该账号保存的密码、Cookie、Token 和 API Key 等凭据。`);
    if (!confirmed) return;
    await runAction("删除账号", () => api(`/api/accounts/${account.id}`, { method: "DELETE" }));
  }

  return (
    <article className={`account-card account-card-v4 ${isProblem ? "is-problem" : ""}`}>
      <div className="account-card-head">
        <div className="account-avatar-stack" aria-label={`${accountDomainLabel(account)}，${upstreamKindLabel(account.upstreamSiteKind || "unknown")}`}>
          <div className="account-avatar" aria-hidden="true">{accountAvatarLabel(account)}</div>
          <span className={`account-kind-chip kind-${account.upstreamSiteKind || "unknown"}`}>{accountBackendShort(account.upstreamSiteKind || "unknown")}</span>
        </div>
        <div className="account-identity">
          <span title={account.upstreamSiteName}>{account.upstreamSiteName}</span>
          <strong title={account.displayName}>{account.displayName}</strong>
          <em title={account.upstreamSiteBaseUrl || "未记录站点地址"}>{account.upstreamSiteBaseUrl || "未记录站点地址"}</em>
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
          <strong>{account.balance !== undefined ? formatBalanceValue(account.balance, account.balanceUnit || "unknown") : "-"}</strong>
        </div>
        <div className="metric-key">
          <span>Key</span>
          <strong>{account.apiKeyFingerprint ? apiKeyStatusLabel(account.apiKeyStatus || "unchecked") : "未保存"}</strong>
        </div>
      </div>

      <div className="chips secondary-chips">
        <span>{account.authType}</span>
        {account.apiKeyFingerprint ? <span>{account.apiKeyFingerprint} · {apiKeyStatusLabel(account.apiKeyStatus || "unchecked")}</span> : <span>未保存密钥</span>}
        {account.lastCheckinAt ? <span>签到 {formatTime(account.lastCheckinAt)}</span> : null}
      </div>

      {account.apiKeyFingerprint ? <AccountKeySummary account={account} /> : null}

      {account.lastCheckinMessage ? <div className="problem-hint">{account.lastCheckinMessage}</div> : null}

      {account.loginStatus === "two_factor_required" && !dismissedTwoFactor ? (
        <TwoFactorGuide
          variant="inline"
          siteName={account.upstreamSiteName}
          baseUrl={account.upstreamSiteBaseUrl}
          loginUrl={account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || "")}
          onClose={() => setDismissedTwoFactor(true)}
          onOpenBrowserLogin={() => void runAction("网页登录", () => api(`/api/accounts/${account.id}/open-browser-login`, { method: "POST" }))}
        />
      ) : null}

      {editing ? (
        <div className="account-card-editor">
          <div className="account-editor-head">
            <strong>账号配置</strong>
            <span>敏感字段留空会保留原值；站点网址变更可选择只改当前账号或同步同站点账号。</span>
          </div>
          <label className="field">
            <span>站点名称</span>
            <input value={siteName} onChange={(event) => setSiteName(event.target.value)} placeholder="站点名称" />
          </label>
          <label className="field">
            <span>后台类型</span>
            <select value={kind} onChange={(event) => setKind(event.target.value)}>
              <option value="auto">自动/保持</option>
              <option value="newapi">NewAPI</option>
              <option value="oneapi">OneAPI</option>
              <option value="sub2api">Sub2API</option>
              <option value="modified_relay">魔改中转</option>
            </select>
          </label>
          <label className="field span-2">
            <span>站点网址</span>
            <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} placeholder="https://example.com" />
          </label>
          <label className="field span-2">
            <span>登录页</span>
            <input value={loginUrl} onChange={(event) => setLoginUrl(event.target.value)} placeholder="默认使用 /login" />
          </label>
          <div className="field span-2">
            <span>站点修改范围</span>
            <div className="segmented scope-segmented">
              <button type="button" className={siteUpdateScope === "current" ? "active" : ""} onClick={() => setSiteUpdateScope("current")}>只改当前账号</button>
              <button type="button" className={siteUpdateScope === "shared" ? "active" : ""} onClick={() => setSiteUpdateScope("shared")}>同步同站点全部账号</button>
            </div>
            <em className="field-help">{siteUpdateScope === "current" ? "适合一个渠道有多个账号时，只修正这张账号卡。" : "会更新这个上游站点，并影响绑定在同一站点下的账号。"}</em>
          </div>
          <label className="field">
            <span>显示名称</span>
            <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="显示名称" />
          </label>
          <label className="field">
            <span>邮箱</span>
            <input value={email} onChange={(event) => setEmail(event.target.value)} placeholder="邮箱账号" />
          </label>
          <label className="field">
            <span>用户名</span>
            <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="非邮箱账号" />
          </label>
          <label className="field">
            <span>认证方式</span>
            <select value={authType} onChange={(event) => setAuthType(event.target.value)}>
              <option value="email_password">账号/邮箱 + 密码</option>
              <option value="api_key">API Key</option>
              <option value="browser_profile">网页登录授权</option>
              <option value="cookie">Cookie</option>
              <option value="access_token">Access Token</option>
            </select>
          </label>
          <label className="field">
            <span>新密码，不填则保留</span>
            <input value={password} onChange={(event) => setPassword(event.target.value)} placeholder="留空不覆盖旧密码" type="password" />
          </label>
          <label className="field">
            <span>新 API Key，不填则保留</span>
            <input value={apiKey} onChange={(event) => setApiKey(event.target.value)} placeholder="留空不覆盖旧密钥" type="password" />
          </label>
          {account.apiKeyFingerprint ? (
            <label className="check">
              <input type="checkbox" checked={clearApiKey} onChange={(event) => setClearApiKey(event.target.checked)} />
              清空当前 API Key
            </label>
          ) : null}
          <div className="toolbar">
            <button type="button" disabled={busy !== ""} onClick={() => void saveAccount()}>{busy === "保存账号" ? "保存中…" : "保存账号"}</button>
            <button type="button" className="ghost" disabled={busy !== ""} onClick={() => setEditing(false)}>取消</button>
          </div>
        </div>
      ) : null}

      <div className="account-card-actions">
        <div className="account-action-group primary">
          <button type="button" aria-label={`为 ${account.displayName} 执行签到`} onClick={() => void runAction("签到", () => api(`/api/accounts/${account.id}/checkin`, { method: "POST" }))}>签到</button>
          <button type="button" aria-label={`刷新 ${account.displayName} 的余额`} onClick={() => void runAction("刷新余额", () => api(`/api/accounts/${account.id}/refresh-balance`, { method: "POST" }))}>刷新余额</button>
          <button type="button" aria-label={`打开 ${account.displayName} 的网页登录`} onClick={() => void runAction("网页登录", () => api(`/api/accounts/${account.id}/open-browser-login`, { method: "POST" }))}>网页登录</button>
          <button type="button" className="ghost" onClick={onOpenDetail}>详情</button>
          <button type="button" className={`ghost more-toggle ${moreOpen ? "active" : ""}`} aria-expanded={moreOpen} onClick={() => setMoreOpen((current) => !current)}>{moreOpen ? "收起" : "更多"}</button>
        </div>
        {moreOpen ? (
          <div className="account-more-panel">
            <div className="account-action-label">维护操作</div>
            <div className="account-action-group secondary">
              <button type="button" className="ghost" onClick={() => setEditing((current) => !current)}>{editing ? "收起编辑" : "编辑账号"}</button>
              <button type="button" className="ghost" onClick={() => void runAction("保存授权", () => api(`/api/accounts/${account.id}/finish-browser-login`, { method: "POST" }))}>保存授权</button>
              <button type="button" className="ghost" onClick={() => void runAction("测试登录态", () => api(`/api/accounts/${account.id}/test-login`, { method: "POST" }))}>测试登录态</button>
              <button type="button" className="ghost" disabled={!account.apiKeyFingerprint || busy !== ""} onClick={() => void testAPIKey()}>{busy === "检测密钥" ? "检测中…" : "检测密钥"}</button>
              <button type="button" className="ghost" onClick={() => setShowTwoFactorGuide(true)}>2FA 指引</button>
            </div>
            <div className="account-action-label danger-label">危险操作</div>
            <div className="account-action-group danger-zone">
              <button type="button" className="danger" onClick={() => void deleteAccount()}>删除账号</button>
            </div>
          </div>
        ) : null}
      </div>
      {message ? <div className={message.includes("失败") || message.includes("错误") ? "error" : "note"}>{message}</div> : null}

      {showTwoFactorGuide ? (
        <TwoFactorGuide
          variant="dialog"
          siteName={account.upstreamSiteName}
          baseUrl={account.upstreamSiteBaseUrl}
          loginUrl={account.upstreamSiteLoginUrl || defaultLoginUrl(account.upstreamSiteBaseUrl || "")}
          onClose={() => setShowTwoFactorGuide(false)}
          onOpenBrowserLogin={() => {
            setShowTwoFactorGuide(false);
            void runAction("网页登录", () => api(`/api/accounts/${account.id}/open-browser-login`, { method: "POST" }));
          }}
        />
      ) : null}
    </article>
  );
}
