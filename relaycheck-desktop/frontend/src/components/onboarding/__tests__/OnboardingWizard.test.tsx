import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import {
  ONBOARDING_CHECKIN_INTENT,
  ONBOARDING_STEPS,
  OnboardingStepIcon,
  onboardingStatusProps,
} from "../OnboardingWizard";

describe("OnboardingWizard accessibility helpers", () => {
  it("renders step icons with the shared SVG icon system", () => {
    const html = renderToStaticMarkup(<OnboardingStepIcon name="sites" />);

    expect(html).toContain("<svg");
    expect(html).toContain("line-icon");
    expect(html).not.toContain("🔗");
    expect(html).not.toContain("📡");
    expect(html).not.toContain("🔑");
    expect(html).not.toContain("✅");
  });

  it("announces success and error feedback with live regions", () => {
    expect(onboardingStatusProps("success")).toEqual({
      role: "status",
      "aria-live": "polite",
      "aria-atomic": true,
    });

    expect(onboardingStatusProps("danger")).toEqual({
      role: "alert",
      "aria-live": "assertive",
      "aria-atomic": true,
    });
  });

  it("uses the current information architecture and delegates safe preview navigation", () => {
    const copy = JSON.stringify(ONBOARDING_STEPS);
    expect(copy).toContain("站点与账号");
    expect(copy).toContain("全部账号");
    expect(copy).not.toContain("左侧账号页");
    expect(ONBOARDING_CHECKIN_INTENT).toEqual({ target: "checkins", checkinPreview: "open" });
  });
});
