import type { Dispatch, SetStateAction } from "react";

import { Button } from "@/components/ui/button";
import { accountActionButtonLabel } from "@/lib/accountActions";

type Scope = "current" | "shared";

export interface AccountCardEditorProps {
  siteName: string;
  setSiteName: Dispatch<SetStateAction<string>>;
  kind: string;
  setKind: Dispatch<SetStateAction<string>>;
  baseUrl: string;
  setBaseUrl: Dispatch<SetStateAction<string>>;
  loginUrl: string;
  setLoginUrl: Dispatch<SetStateAction<string>>;
  siteUpdateScope: Scope;
  setSiteUpdateScope: Dispatch<SetStateAction<Scope>>;
  displayName: string;
  setDisplayName: Dispatch<SetStateAction<string>>;
  email: string;
  setEmail: Dispatch<SetStateAction<string>>;
  username: string;
  setUsername: Dispatch<SetStateAction<string>>;
  authType: string;
  setAuthType: Dispatch<SetStateAction<string>>;
  password: string;
  setPassword: Dispatch<SetStateAction<string>>;
  apiKey: string;
  setApiKey: Dispatch<SetStateAction<string>>;
  hasAPIKey: boolean;
  clearApiKey: boolean;
  setClearApiKey: Dispatch<SetStateAction<boolean>>;
  busy: string;
  isBusy: boolean;
  onSave: () => Promise<void>;
  onCancel: () => void;
}

export function AccountCardEditor(props: AccountCardEditorProps) {
  return (
    <div className="account-card-editor">
      <div className="account-editor-head">
        <strong>账号配置</strong>
        <span>敏感字段留空会保留原值；站点网址变更可选择只改当前账号或同步同站点账号。</span>
      </div>
      <label className="field">
        <span>站点名称</span>
        <input
          value={props.siteName}
          onChange={(event) => props.setSiteName(event.target.value)}
          placeholder="站点名称"
        />
      </label>
      <label className="field">
        <span>后台类型</span>
        <select value={props.kind} onChange={(event) => props.setKind(event.target.value)}>
          <option value="auto">自动/保持</option>
          <option value="newapi">NewAPI</option>
          <option value="oneapi">OneAPI</option>
          <option value="sub2api">Sub2API</option>
          <option value="modified_relay">魔改中转</option>
        </select>
      </label>
      <label className="field span-2">
        <span>站点网址</span>
        <input
          value={props.baseUrl}
          onChange={(event) => props.setBaseUrl(event.target.value)}
          placeholder="https://example.com"
        />
      </label>
      <label className="field span-2">
        <span>登录页</span>
        <input
          value={props.loginUrl}
          onChange={(event) => props.setLoginUrl(event.target.value)}
          placeholder="默认使用 /login"
        />
      </label>
      <div className="field span-2">
        <span>站点修改范围</span>
        <div className="segmented scope-segmented">
          <button
            type="button"
            className={props.siteUpdateScope === "current" ? "active" : ""}
            onClick={() => props.setSiteUpdateScope("current")}
          >
            只改当前账号
          </button>
          <button
            type="button"
            className={props.siteUpdateScope === "shared" ? "active" : ""}
            onClick={() => props.setSiteUpdateScope("shared")}
          >
            同步同站点全部账号
          </button>
        </div>
        <em className="field-help">
          {props.siteUpdateScope === "current"
            ? "适合一个渠道有多个账号时，只修正这张账号卡。"
            : "会更新这个上游站点，并影响绑定在同一站点下的账号。"}
        </em>
      </div>
      <label className="field">
        <span>显示名称</span>
        <input
          value={props.displayName}
          onChange={(event) => props.setDisplayName(event.target.value)}
          placeholder="显示名称"
        />
      </label>
      <label className="field">
        <span>邮箱</span>
        <input value={props.email} onChange={(event) => props.setEmail(event.target.value)} placeholder="邮箱账号" />
      </label>
      <label className="field">
        <span>用户名</span>
        <input
          value={props.username}
          onChange={(event) => props.setUsername(event.target.value)}
          placeholder="非邮箱账号"
        />
      </label>
      <label className="field">
        <span>认证方式</span>
        <select value={props.authType} onChange={(event) => props.setAuthType(event.target.value)}>
          <option value="email_password">账号/邮箱 + 密码</option>
          <option value="api_key">API Key</option>
          <option value="browser_profile">网页登录授权</option>
          <option value="cookie">Cookie</option>
          <option value="access_token">Access Token</option>
        </select>
      </label>
      <label className="field">
        <span>新密码，不填则保留</span>
        <input
          value={props.password}
          onChange={(event) => props.setPassword(event.target.value)}
          placeholder="留空不覆盖旧密码"
          type="password"
        />
      </label>
      <label className="field">
        <span>新 API Key，不填则保留</span>
        <input
          value={props.apiKey}
          onChange={(event) => props.setApiKey(event.target.value)}
          placeholder="留空不覆盖旧密钥"
          type="password"
        />
      </label>
      {props.hasAPIKey ? (
        <label className="check">
          <input
            type="checkbox"
            checked={props.clearApiKey}
            onChange={(event) => props.setClearApiKey(event.target.checked)}
          />
          清空当前 API Key
        </label>
      ) : null}
      <div className="toolbar">
        <button type="button" disabled={props.isBusy} onClick={() => void props.onSave()}>
          {accountActionButtonLabel("保存账号", props.busy, "保存中…")}
        </button>
        <Button variant="ghost" type="button" disabled={props.isBusy} onClick={props.onCancel}>
          取消
        </Button>
      </div>
    </div>
  );
}
