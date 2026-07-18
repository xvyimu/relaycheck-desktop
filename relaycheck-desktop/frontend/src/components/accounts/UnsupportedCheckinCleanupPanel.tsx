import type { UnsupportedCheckinCleanupResult } from "@/api/account-cleanup";
import { cleanupReasonLabel } from "@/components/accounts/accountInsightsUtils";
import { Button } from "@/components/ui/button";

const DEFAULT_CLEANUP_LIMIT = 10;

export interface UnsupportedCheckinCleanupPanelProps {
  preview: UnsupportedCheckinCleanupResult | null;
  busy: boolean;
  includeLastUnsupported: boolean;
  initialMatched?: number;
  onPreview: () => void | Promise<void>;
  onConfirm: (previewId: string) => void | Promise<void>;
  onIncludeLastUnsupportedChange: (checked: boolean) => void;
  onClearPreview: () => void;
}

/** 根据当前预览状态生成稳定的预览按钮文案。 */
function cleanupPreviewButtonLabel(preview: UnsupportedCheckinCleanupResult | null, busy: boolean): string {
  if (busy) return "处理中";
  if (preview?.deleted) return preview.hasMore ? "继续预览下一批" : "再次检查";
  return preview ? "重新预览" : "预览清理";
}

/** 根据预览和执行结果生成紧凑状态标签。 */
function cleanupStatusLabel(preview: UnsupportedCheckinCleanupResult | null): string {
  if (!preview) return "先预览";
  if (preview.deleted) return preview.hasMore ? "还有下一批" : "已清理";
  if (!preview.matched) return "无需清理";
  return preview.hasMore ? "等待确认+" : "等待确认";
}

/** UnsupportedCheckinCleanupPanel 只负责展示冻结候选和收集用户二次确认，不直接访问 API。 */
export function UnsupportedCheckinCleanupPanel({
  preview,
  busy,
  includeLastUnsupported,
  initialMatched = 0,
  onPreview,
  onConfirm,
  onIncludeLastUnsupportedChange,
  onClearPreview,
}: UnsupportedCheckinCleanupPanelProps) {
  const batchLimit = preview?.limit || DEFAULT_CLEANUP_LIMIT;
  const canDelete = Boolean(preview?.matched && preview.deleted === 0 && preview.previewId);

  /** 二次确认仅传递当前 previewId；取消时不调用 owner 的确认函数。 */
  async function confirmCurrentPreview() {
    if (!preview?.previewId || !canDelete) return;
    const samples = preview.items
      .slice(0, 3)
      .map((item) => item.upstreamSiteName + " / " + item.accountName)
      .join("、");
    const confirmed = window.confirm(
      "确认删除 " +
        preview.matched +
        " 个不支持签到的账号？这会同步删除这些账号的签到日志和余额快照。" +
        (samples ? "\n样例：" + samples : ""),
    );
    if (!confirmed) return;
    // owner 只接收服务端签发的 token，无法重新选择或扩大候选集合。
    await onConfirm(preview.previewId);
  }

  return (
    <div className="account-capability-panel unsupported-cleanup-panel is-actionable">
      <div className="capability-panel-head">
        <div>
          <span>签到清理</span>
          <strong>{preview?.matched ?? initialMatched}</strong>
        </div>
        <em>{cleanupStatusLabel(preview)}</em>
      </div>
      <label className="cleanup-option">
        <input
          type="checkbox"
          checked={includeLastUnsupported}
          onChange={(event) => {
            // 过滤范围变化会使旧 previewId 失效，必须先清空再允许重新预览。
            onIncludeLastUnsupportedChange(event.currentTarget.checked);
            onClearPreview();
          }}
        />
        包含上次签到返回“不支持”的账号
      </label>
      <div className="capability-list cleanup-preview-list">
        {(preview?.items || []).slice(0, 5).map((item) => (
          <div className="capability-row issue-row" key={"cleanup-" + item.accountId}>
            <div>
              <strong title={item.accountName}>{item.accountName}</strong>
              <span title={item.upstreamSiteName + " · " + item.upstreamSiteKind}>
                {item.upstreamSiteName} · {cleanupReasonLabel(item.reason)}
              </span>
            </div>
            <b>{item.lastCheckinStatus || "site"}</b>
          </div>
        ))}
        {preview && preview.items.length > 5 ? (
          <span className="capability-empty">
            还有 {preview.items.length - 5} 个账号未展开；本批接口最多处理 {batchLimit} 个。
          </span>
        ) : null}
        {preview?.hasMore ? (
          <span className="capability-empty">
            后面还有下一批候选账号；当前批次上限 {batchLimit} 个，删除后请继续预览。
          </span>
        ) : null}
        {preview && preview.deleted > 0 ? (
          <span className="capability-empty">
            本批已通过 API 删除 {preview.deleted} 个账号；再次预览会读取下一批或确认已归零。
          </span>
        ) : null}
        {!preview ? <span className="capability-empty">先预览将要删除的账号；预览模式不会写入数据库。</span> : null}
        {preview && !preview.items.length ? (
          <span className="capability-empty">当前没有匹配的不支持签到账号。</span>
        ) : null}
      </div>
      <div className="mini-action-row">
        <Button variant="ghost" type="button" disabled={busy} onClick={() => void onPreview()}>
          {cleanupPreviewButtonLabel(preview, busy)}
        </Button>
        <button
          type="button"
          className="danger"
          disabled={busy || !canDelete}
          onClick={() => void confirmCurrentPreview()}
        >
          删除本批
        </button>
      </div>
    </div>
  );
}
