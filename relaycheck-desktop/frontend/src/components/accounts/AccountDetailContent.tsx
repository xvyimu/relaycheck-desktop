import { formatBalanceValue, formatTime } from "@/lib/format";
import { apiKeyStatusLabel, loginStatusLabel, statusLabel } from "@/lib/labels";
import type { Account } from "@/types";

export function AccountDetailContent({ account, onClose }: { account: Account; onClose: () => void }) {
  const identity = account.email || account.username || account.authType;
  const checkinState = account.lastCheckinStatus || "unknown";
  const keyState = account.apiKeyFingerprint ? apiKeyStatusLabel(account.apiKeyStatus || "unchecked") : "未保存";
  return (
    <>
      <div className="detail-header">
        <div>
          <span className="eyebrow">账号详情</span>
          <strong>{account.displayName}</strong>
          <p>{account.upstreamSiteName} · {account.upstreamSiteBaseUrl || "未记录站点地址"}</p>
        </div>
        <div className="detail-actions">
          <button className="ghost" onClick={onClose}>关闭</button>
        </div>
      </div>

      <div className="detail-grid">
        <section className="detail-card">
          <h3>运营状态</h3>
          <div className="detail-metrics">
            <div><span>登录态</span><strong>{loginStatusLabel(account.loginStatus)}</strong></div>
            <div><span>签到</span><strong>{statusLabel(checkinState)}</strong></div>
            <div><span>余额</span><strong>{account.balance !== undefined ? formatBalanceValue(account.balance, account.balanceUnit || "unknown") : "-"}</strong></div>
            <div><span>Key</span><strong>{keyState}</strong></div>
          </div>
          <div className="detail-list">
            <div><span>账号标识</span><strong>{identity}</strong></div>
            <div><span>认证方式</span><strong>{account.authType}</strong></div>
            <div><span>最近签到</span><strong>{formatTime(account.lastCheckinAt || "")}</strong></div>
            <div><span>最近登录验证</span><strong>{formatTime(account.lastValidatedAt || "")}</strong></div>
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
          <div className="signal-list">
            {account.apiKeySampleModels?.length ? account.apiKeySampleModels.slice(0, 8).map((model) => <span key={model}>{model}</span>) : <span>暂无模型样本</span>}
          </div>
          {account.apiKeyTestMessage ? <div className="problem-hint detail-hint">{account.apiKeyTestMessage}</div> : null}
        </section>

        <section className="detail-card">
          <h3>建议动作</h3>
          <div className="detail-stack">
            {account.loginStatus !== "valid" ? <div className="problem-hint detail-hint">登录态不是有效状态，优先执行网页登录或保存授权，再重新签到。</div> : null}
            {!["success", "already_checked"].includes(checkinState) ? <div className="problem-hint detail-hint">最近签到未确认成功，建议在签到页查看返回消息，避免误判为成功。</div> : null}
            {account.apiKeyFingerprint && account.apiKeyStatus !== "valid" ? <div className="problem-hint detail-hint">API Key 需要重新检测，模型列表和价格判断可能不完整。</div> : null}
            {account.balance === undefined ? <div className="problem-hint detail-hint">还没有余额快照，刷新余额后才能做低余额和消耗趋势判断。</div> : null}
            {account.loginStatus === "valid" && ["success", "already_checked"].includes(checkinState) && (!account.apiKeyFingerprint || account.apiKeyStatus === "valid") ? <div className="note">账号状态清爽，可以优先处理其他异常账号。</div> : null}
          </div>
        </section>
      </div>
    </>
  );
}