import { formatBalanceValue, formatTime } from "@/lib/format";
import { apiKeyStatusLabel, loginStatusLabel, statusLabel } from "@/lib/labels";
import type { Account } from "@/types";
import { TwoFactorGuide } from "@/components/ui/TwoFactorGuide";

export function AccountDetailContent({ account, onClose }: { account: Account; onClose: () => void }) {
  const identity = account.email || account.username || account.authType;
  const checkinState = account.lastCheckinStatus || "";
  const keyState = account.apiKeyFingerprint ? apiKeyStatusLabel(account.apiKeyStatus || "unchecked") : "未保存";
  const needsTwoFactor = account.loginStatus === "two_factor_required";
  return (
    <>
      <div className="detail-header">
        <div>
          <span className="eyebrow">账号详情</span>
          <strong>{account.displayName}</strong>
          <p>{account.upstreamSiteName || "未记录站点"}</p>
        </div>
        <div className="detail-actions">
          <button className="ghost" onClick={onClose}>关闭</button>
        </div>
      </div>

      <div className="detail-grid">
        <section className="detail-card">
          <h3>运营状态</h3>
          <div className="detail-metrics">
            <div><span>登录</span><strong>{loginStatusLabel(account.loginStatus)}</strong></div>
            <div><span>签到</span><strong>{statusLabel(checkinState)}</strong></div>
            <div><span>余额</span><strong>{account.balance !== undefined ? formatBalanceValue(account.balance, account.balanceUnit || "unknown") : "-"}</strong></div>
          </div>
          <div className="detail-list">
            <div><span>标识</span><strong>{identity}</strong></div>
            <div><span>认证</span><strong>{account.authType}</strong></div>
            <div><span>最近签到</span><strong>{formatTime(account.lastCheckinAt || "")}</strong></div>
            <div><span>验证时间</span><strong>{formatTime(account.lastValidatedAt || "")}</strong></div>
          </div>
        </section>

        <section className="detail-card">
          <h3>Key 与模型</h3>
          <div className="detail-list">
            <div><span>指纹</span><strong>{account.apiKeyFingerprint || "未保存"}</strong></div>
            <div><span>检测状态</span><strong>{keyState}</strong></div>
            <div><span>测试模型</span><strong>{account.apiKeyTestModel || "未测速"}</strong></div>
            <div><span>延迟</span><strong>{account.apiKeyLatencyMs ? `${account.apiKeyLatencyMs}ms` : "未测速"}</strong></div>
          </div>
          {account.apiKeySampleModels?.length ? (
            <div className="signal-list">
              {account.apiKeySampleModels.slice(0, 8).map((model) => <span key={model}>{model}</span>)}
            </div>
          ) : null}
          {account.apiKeyTestMessage ? <div className="problem-hint detail-hint">{account.apiKeyTestMessage}</div> : null}
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
            {account.loginStatus !== "valid" && !needsTwoFactor ? <div className="problem-hint detail-hint">登录态异常，需重新登录或保存授权。</div> : null}
            {!["success", "already_checked"].includes(checkinState) ? <div className="problem-hint detail-hint">最近签到未确认成功，建议在签到页查看返回消息。</div> : null}
            {account.apiKeyFingerprint && account.apiKeyStatus !== "valid" ? <div className="problem-hint detail-hint">API Key 状态异常，需要重新检测。</div> : null}
            {account.balance === undefined ? <div className="problem-hint detail-hint">暂无余额快照，刷新余额后再做趋势判断。</div> : null}
            {account.loginStatus === "valid" && ["success", "already_checked"].includes(checkinState) && (!account.apiKeyFingerprint || account.apiKeyStatus === "valid") ? <div className="note">账号状态正常，已是最佳状态。</div> : null}
          </div>
        </section>
      </div>
    </>
  );
}