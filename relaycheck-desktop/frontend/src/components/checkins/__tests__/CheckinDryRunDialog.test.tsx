import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { CheckinDryRunPreview } from "@/api/checkins";
import { CheckinDryRunDialog } from "../CheckinDryRunDialog";

function makePreview(count = 2): CheckinDryRunPreview {
  return {
    type: "checkin",
    previewId: "preview-1",
    expiresAt: "2026-07-18T01:05:00Z",
    maxAccounts: 200,
    totalAccounts: count,
    willRun: Math.max(0, count - 1),
    skipped: count > 0 ? 1 : 0,
    items: Array.from({ length: count }, (_, index) => ({
      accountId: `account-${index + 1}`,
      accountName: `账号 ${index + 1}`,
      siteName: `站点 ${index + 1}`,
      action: index === count - 1 ? "skip_missing_credentials" : "will_run",
      reason: index === count - 1 ? "缺少本地认证信息" : "将尝试签到",
    })),
  };
}

const callbacks = {
  onClose: vi.fn(),
  onRetry: vi.fn(),
  onConfirm: vi.fn(),
  onFixAccounts: vi.fn(),
};

describe("CheckinDryRunDialog", () => {
  it("renders counts, skip reasons and only the first 12 items", () => {
    const html = renderToStaticMarkup(
      <CheckinDryRunDialog open preview={makePreview(14)} loading={false} starting={false} error="" {...callbacks} />,
    );

    expect(html).toContain("将尝试执行");
    expect(html).toContain("跳过");
    expect(html).toContain("缺少本地认证信息");
    expect(html).toContain("账号 12");
    expect(html).not.toContain("账号 13");
    expect(html).toContain("另有 2 条");
  });

  it("disables confirmation and offers credential repair when nothing can run", () => {
    const preview = makePreview(1);
    preview.willRun = 0;
    preview.previewId = undefined;
    const html = renderToStaticMarkup(
      <CheckinDryRunDialog open preview={preview} loading={false} starting={false} error="" {...callbacks} />,
    );

    expect(html).toContain("没有可执行账号");
    expect(html).toContain("前往站点与账号");
    expect(html).toMatch(/<button[^>]*disabled[^>]*>确认执行<\/button>/);
  });

  it("announces recoverable errors and keeps retry available", () => {
    const html = renderToStaticMarkup(
      <CheckinDryRunDialog
        open
        preview={null}
        loading={false}
        starting={false}
        error="预览失败，请重试"
        {...callbacks}
      />,
    );

    expect(html).toContain('role="alert"');
    expect(html).toContain("预览失败，请重试");
    expect(html).toContain("重新预览");
  });
});
