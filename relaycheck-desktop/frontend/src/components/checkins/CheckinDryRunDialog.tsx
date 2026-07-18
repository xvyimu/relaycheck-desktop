import type { CheckinDryRunPreview } from "@/api/checkins";
import { Button } from "@/components/ui/button";
import { DialogShell } from "@/components/ui/dialog-shell";
import { LineIcon } from "@/components/ui/line-icon";

const previewDisplayLimit = 12;

type CheckinDryRunDialogProps = {
  open: boolean;
  preview: CheckinDryRunPreview | null;
  loading: boolean;
  starting: boolean;
  error: string;
  onClose: () => void;
  onRetry: () => void;
  onConfirm: () => void;
  onFixAccounts: () => void;
};

function actionLabel(action: CheckinDryRunPreview["items"][number]["action"]) {
  return action === "will_run" ? "将执行" : "跳过";
}

export function CheckinDryRunDialog({
  open,
  preview,
  loading,
  starting,
  error,
  onClose,
  onRetry,
  onConfirm,
  onFixAccounts,
}: CheckinDryRunDialogProps) {
  const canConfirm = Boolean(preview?.previewId && preview.willRun > 0 && !loading && !starting);
  const visibleItems = preview?.items.slice(0, previewDisplayLimit) ?? [];
  const hiddenCount = Math.max(0, (preview?.items.length ?? 0) - visibleItems.length);
  const skippedReasons = Array.from(
    new Set(preview?.items.filter((item) => item.action !== "will_run").map((item) => item.reason) ?? []),
  ).slice(0, 3);

  return (
    <DialogShell
      open={open}
      onClose={() => {
        if (!starting) onClose();
      }}
      closeOnBackdrop={!starting}
      variant="modal"
      titleId="checkin-preview-title"
      className="checkin-preview-dialog"
      backdropClassName="checkin-preview-backdrop"
      initialFocusSelector={error ? "[data-preview-retry]" : undefined}
    >
      <header className="checkin-preview-header">
        <div>
          <span className="checkin-preview-eyebrow">批量签到安全预览</span>
          <h2 id="checkin-preview-title">确认将尝试执行的账号</h2>
        </div>
        <Button variant="ghost" type="button" onClick={onClose} disabled={starting} aria-label="关闭签到预览">
          关闭
        </Button>
      </header>

      <div className="checkin-preview-body">
        {loading ? (
          <div className="checkin-preview-loading" role="status" aria-live="polite">
            正在获取同源候选账号…
          </div>
        ) : null}
        {error ? (
          <div className="checkin-preview-error" role="alert" aria-live="assertive">
            <LineIcon name="danger" />
            <span>{error}</span>
          </div>
        ) : null}

        {preview ? (
          <>
            <dl className="checkin-preview-counts" aria-label="签到预览统计">
              <div>
                <dt>候选账号</dt>
                <dd>{preview.totalAccounts}</dd>
              </div>
              <div>
                <dt>将尝试执行</dt>
                <dd>{preview.willRun}</dd>
              </div>
              <div>
                <dt>跳过</dt>
                <dd>{preview.skipped}</dd>
              </div>
            </dl>

            {skippedReasons.length > 0 ? (
              <div className="checkin-preview-reasons">
                <strong>主要跳过原因</strong>
                <span>{skippedReasons.join("；")}</span>
              </div>
            ) : null}

            {preview.willRun === 0 ? (
              <div className="checkin-preview-empty" role="status">
                <strong>没有可执行账号</strong>
                <span>请先补充凭据或确认站点具备签到能力。</span>
              </div>
            ) : null}

            <div className="checkin-preview-list" aria-label="签到预览账号">
              {visibleItems.map((item) => (
                <div className="checkin-preview-item" key={item.accountId}>
                  <div>
                    <strong>{item.accountName || "未知账号"}</strong>
                    <span>{item.siteName || "未知站点"}</span>
                  </div>
                  <div>
                    <span className={`status-pill ${item.action === "will_run" ? "success" : "warning"}`}>
                      {actionLabel(item.action)}
                    </span>
                    <span>{item.reason}</span>
                  </div>
                </div>
              ))}
            </div>
            {hiddenCount > 0 ? <p className="checkin-preview-more">另有 {hiddenCount} 条未展开</p> : null}
          </>
        ) : null}
      </div>

      <footer className="checkin-preview-actions">
        <Button variant="ghost" type="button" onClick={onClose} disabled={starting}>
          取消
        </Button>
        {error ? (
          <Button type="button" onClick={onRetry} disabled={loading || starting} data-preview-retry>
            重新预览
          </Button>
        ) : preview?.willRun === 0 ? (
          <>
            <Button type="button" onClick={onFixAccounts}>
              前往站点与账号
            </Button>
            <button type="button" disabled>
              确认执行
            </button>
          </>
        ) : (
          <button type="button" onClick={onConfirm} disabled={!canConfirm}>
            {starting ? "启动中…" : "确认执行"}
          </button>
        )}
      </footer>
    </DialogShell>
  );
}
