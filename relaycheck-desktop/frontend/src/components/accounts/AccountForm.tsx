import { useEffect, useMemo, useState } from "react";
import { api } from "@/api/client";
import type { UpstreamSite } from "@/types";

export function AccountForm({ sites, onDone }: { sites: UpstreamSite[]; onDone: () => void }) {
  const [siteMode, setSiteMode] = useState<"existing" | "custom">("existing");
  const [upstreamSiteId, setUpstreamSiteId] = useState("");
  const [siteName, setSiteName] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [loginUrl, setLoginUrl] = useState("");
  const [kind, setKind] = useState("auto");
  const [authType, setAuthType] = useState("email_password");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const siteOptions = useMemo(() => sites, [sites]);
  const isCustomSite = siteMode === "custom";
  const canSubmit = isCustomSite ? baseUrl.trim() !== "" : upstreamSiteId !== "";

  useEffect(() => {
    if (siteMode === "existing" && !upstreamSiteId && siteOptions[0]) setUpstreamSiteId(siteOptions[0].id);
  }, [siteMode, siteOptions, upstreamSiteId]);

  return (
    <form
      className="card account-create-card"
      onSubmit={async (event) => {
        event.preventDefault();
        if (!canSubmit || busy) return;
        setBusy(true);
        setMessage("");
        try {
          await api("/api/accounts", {
            method: "POST",
            body: JSON.stringify({
              upstreamSiteId: isCustomSite ? "" : upstreamSiteId,
              siteName,
              baseUrl: isCustomSite ? baseUrl : "",
              loginUrl: isCustomSite ? loginUrl : "",
              kind: kind === "auto" ? "" : kind,
              displayName,
              email,
              username,
              password,
              apiKey,
              authType,
            }),
          });
          setDisplayName("");
          setEmail("");
          setUsername("");
          setPassword("");
          setApiKey("");
          setMessage("账号已添加。可以直接测试登录态、刷新余额，或点击网页登录保存授权。");
          await onDone();
        } catch (error) {
          setMessage(error instanceof Error ? error.message : "账号添加失败");
        } finally {
          setBusy(false);
        }
      }}
    >
      <div className="section-heading">
        <div>
          <strong>添加账号 / 自定义站点</strong>
          <span>同一个中转站可以绑定多个账号；填新网址时会先识别 NewAPI / OneAPI / Sub2API，再创建账号。</span>
        </div>
        <div className="segmented">
          <button type="button" className={siteMode === "existing" ? "active" : ""} onClick={() => setSiteMode("existing")}>已有站点</button>
          <button type="button" className={siteMode === "custom" ? "active" : ""} onClick={() => setSiteMode("custom")}>自定义网址</button>
        </div>
      </div>

      <div className="account-form-grid">
        {isCustomSite ? (
          <>
            <label className="field span-2">
              <span>站点网址</span>
              <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} placeholder="https://example.com" />
            </label>
            <label className="field">
              <span>站点名称</span>
              <input value={siteName} onChange={(event) => setSiteName(event.target.value)} placeholder="可自动用域名" />
            </label>
            <label className="field">
              <span>后台类型</span>
              <select value={kind} onChange={(event) => setKind(event.target.value)}>
                <option value="auto">自动识别</option>
                <option value="newapi">NewAPI</option>
                <option value="oneapi">OneAPI</option>
                <option value="sub2api">Sub2API</option>
                <option value="modified_relay">魔改中转</option>
              </select>
            </label>
            <label className="field span-2">
              <span>登录页，可选</span>
              <input value={loginUrl} onChange={(event) => setLoginUrl(event.target.value)} placeholder="默认使用 /login" />
            </label>
          </>
        ) : (
          <label className="field span-2">
            <span>选择站点</span>
            <select value={upstreamSiteId} onChange={(event) => setUpstreamSiteId(event.target.value)}>
              {siteOptions.map((site) => (
                <option key={site.id} value={site.id}>
                  {site.name} · {site.baseUrl}
                </option>
              ))}
            </select>
          </label>
        )}
        <label className="field">
          <span>认证方式</span>
          <select value={authType} onChange={(event) => setAuthType(event.target.value)}>
            <option value="email_password">账号/邮箱 + 密码</option>
            <option value="api_key">API Key</option>
            <option value="browser_profile">网页登录授权</option>
          </select>
        </label>
        <label className="field">
          <span>显示名称</span>
          <input value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="可留空自动生成" />
        </label>
        <label className="field">
          <span>邮箱</span>
          <input value={email} onChange={(event) => setEmail(event.target.value)} placeholder="邮箱账号" />
        </label>
        <label className="field">
          <span>用户名</span>
          <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="非邮箱账号可填这里" />
        </label>
        <label className="field">
          <span>密码</span>
          <input value={password} onChange={(event) => setPassword(event.target.value)} placeholder="账号密码，可选" type="password" />
        </label>
        <label className="field span-2">
          <span>API Key，可选</span>
          <input value={apiKey} onChange={(event) => setApiKey(event.target.value)} placeholder="用于区分同站点不同密钥账号，也可后续检测是否有效" type="password" />
        </label>
      </div>

      <div className="toolbar">
        <button type="submit" disabled={!canSubmit || busy}>{busy ? "添加中..." : "添加账号"}</button>
        <span className="muted">{isCustomSite ? "只允许添加识别为 NewAPI / OneAPI / Sub2API 的面板型中转站。" : "已有站点会直接绑定新账号，不覆盖旧账号。"}</span>
      </div>
      {message ? <div className={message.includes("失败") || message.includes("未识别") ? "error" : "note"}>{message}</div> : null}
    </form>
  );
}