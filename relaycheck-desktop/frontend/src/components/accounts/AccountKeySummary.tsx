import { formatTime } from "@/lib/format";
import { apiKeyStatusLabel } from "@/lib/labels";
import type { Account } from "@/types";

export function AccountKeySummary({ account }: { account: Account }) {
  const status = account.apiKeyStatus || "unchecked";
  const models = account.apiKeySampleModels || [];
  const modelLabel = account.apiKeyModelCount && account.apiKeyModelCount > 0 ? `${account.apiKeyModelCount} 个` : "未获取";
  const speedLabel = account.apiKeyLatencyMs && account.apiKeyLatencyMs > 0 ? `${account.apiKeyLatencyMs}ms` : "未测速";
  const usableLabel = account.apiKeyTestModel ? (account.apiKeyModelUsable ? "模型可用" : "模型不可用") : "待测试";
  const sampleLabel = models.length ? models.slice(0, 3).join("、") : "无样例";

  return (
    <div className={`account-key-summary key-${status} ${account.apiKeyModelUsable ? "is-usable" : ""}`}>
      <div>
        <span>Key 状态</span>
        <strong>{apiKeyStatusLabel(status)}</strong>
        <em>{account.apiKeyLastCheckedAt ? formatTime(account.apiKeyLastCheckedAt) : "未检测"}</em>
      </div>
      <div>
        <span>模型</span>
        <strong>{modelLabel}</strong>
        <em title={sampleLabel}>{sampleLabel}</em>
      </div>
      <div>
        <span>测速</span>
        <strong title={account.apiKeyTestModel || "未测速"}>{account.apiKeyTestModel || "未测速"}</strong>
        <em>{speedLabel} · {usableLabel}</em>
      </div>
      {account.apiKeyTestMessage ? <p title={account.apiKeyTestMessage}>{account.apiKeyTestMessage}</p> : null}
    </div>
  );
}