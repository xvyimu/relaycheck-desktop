import type { NetworkProxyConfig, ProxyTestResult } from "@/types";
import { Button } from "@/components/ui/button";
import { StatusLabel } from "@/components/ui/status-label";

export type SettingsProxyProps = {
  proxyConfig: NetworkProxyConfig;
  proxyTestTarget: string;
  proxyTestResult: ProxyTestResult | null;
  busy: boolean;
  canSave: boolean;
  defaultConfig: NetworkProxyConfig;
  onPatch: (patch: Partial<NetworkProxyConfig>) => void;
  onTargetChange: (value: string) => void;
  onTest: () => void;
  onReset: () => void;
};

export function SettingsProxy({
  proxyConfig,
  proxyTestTarget,
  proxyTestResult,
  busy,
  canSave,
  onPatch,
  onTargetChange,
  onTest,
  onReset,
}: SettingsProxyProps) {
  return (
    <article className="card settings-proxy-card">
      <div className="section-heading">
        <div>
          <strong>网络代理</strong>
          <span>用于外部中转站探测、签到、余额刷新和 API Key 检测。本地 127.0.0.1 默认直连。</span>
        </div>
        <span className={"status-pill " + (proxyConfig.enabled ? "success" : "neutral")}>
          <StatusLabel
            level={proxyConfig.enabled ? "enabled" : "disabled"}
            label={proxyConfig.enabled ? "已启用" : "未启用"}
          />
        </span>
      </div>
      <div className="proxy-toggle-row">
        <label className="check">
          <input
            type="checkbox"
            checked={proxyConfig.enabled}
            onChange={(event) => onPatch({ enabled: event.target.checked })}
          />
          启用代理
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={proxyConfig.bypassLocal}
            onChange={(event) => onPatch({ bypassLocal: event.target.checked })}
          />
          绕过本地地址
        </label>
      </div>
      <div className="proxy-form-grid">
        <label className="field">
          <span>代理地址</span>
          <input
            value={proxyConfig.url}
            onChange={(event) => onPatch({ url: event.target.value })}
            placeholder="http://127.0.0.1:7897"
          />
        </label>
        <label className="field">
          <span>测试地址</span>
          <input
            value={proxyTestTarget}
            onChange={(event) => onTargetChange(event.target.value)}
            placeholder="https://wxls.ccwu.cc/"
          />
        </label>
      </div>
      <div className="proxy-actions">
        <button disabled={busy || !canSave} onClick={() => void onTest()}>
          {busy ? "测试中…" : "保存并测试代理"}
        </button>
        <Button variant="ghost" disabled={busy} onClick={onReset}>
          恢复默认
        </Button>
      </div>
      {proxyTestResult ? (
        <div className={"proxy-result " + (proxyTestResult.ok ? "success" : "warning")}>
          <strong>
            <StatusLabel
              level={proxyTestResult.ok ? "success" : "warning"}
              label={proxyTestResult.ok ? "连通" : "未连通"}
            />
          </strong>
          <span>
            {proxyTestResult.targetUrl} {"·"}{" "}
            {proxyTestResult.httpStatus ? "HTTP " + proxyTestResult.httpStatus + " · " : ""}
            {proxyTestResult.latencyMs}ms
          </span>
          <p>{proxyTestResult.message}</p>
        </div>
      ) : (
        <div className="problem-hint detail-hint">
          如果某些站点 Chrome 能打开但工具检测失败，先开启这里的代理并测试目标站点。
        </div>
      )}
    </article>
  );
}
