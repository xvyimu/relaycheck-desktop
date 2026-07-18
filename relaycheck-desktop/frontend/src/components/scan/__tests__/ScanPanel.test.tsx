import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { hasSuccessfulScanImport, scanFeedbackProps, ScanPanel } from "../ScanPanel";

describe("ScanPanel accessibility", () => {
  it("exposes idle scan state and empty-state announcement", () => {
    const html = renderToStaticMarkup(<ScanPanel onRefresh={async () => undefined} onNavigate={() => undefined} />);

    expect(html).toContain('aria-busy="false"');
    expect(html).toContain('aria-label="本机 NewAPI 扫描"');
    expect(html).toContain('role="status"');
    expect(html).toContain('aria-live="polite"');
  });

  it("announces scan results with the right urgency", () => {
    expect(scanFeedbackProps(false)).toEqual({
      role: "status",
      "aria-live": "polite",
      "aria-atomic": true,
    });

    expect(scanFeedbackProps(true)).toEqual({
      role: "alert",
      "aria-live": "assertive",
      "aria-atomic": true,
    });
  });

  it("shows next steps only for successful and mixed imports", () => {
    const success = {
      found: true,
      message: "导入完成",
      results: [{ dbPath: "one.db", baseUrl: "http://127.0.0.1", importedCount: 1, sitesCreated: 0, sitesMerged: 0 }],
    };
    const mixed = {
      ...success,
      results: [...success.results, { ...success.results[0], dbPath: "two.db", importedCount: 0, error: "读取失败" }],
    };
    const failed = {
      found: true,
      message: "导入失败",
      results: [{ ...success.results[0], importedCount: 0, error: "读取失败" }],
    };
    const empty = { found: true, message: "未导入", results: [{ ...success.results[0], importedCount: 0 }] };

    expect(hasSuccessfulScanImport(success)).toBe(true);
    expect(hasSuccessfulScanImport(mixed)).toBe(true);
    expect(hasSuccessfulScanImport(failed)).toBe(false);
    expect(hasSuccessfulScanImport(empty)).toBe(false);
    expect(hasSuccessfulScanImport({ ...success, found: false })).toBe(false);
  });
});
