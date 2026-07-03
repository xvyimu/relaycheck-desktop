import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { scanFeedbackProps, ScanPanel } from "../ScanPanel";

describe("ScanPanel accessibility", () => {
  it("exposes idle scan state and empty-state announcement", () => {
    const html = renderToStaticMarkup(<ScanPanel onRefresh={async () => undefined} />);

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
});
