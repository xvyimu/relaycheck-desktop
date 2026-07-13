import { describe, expect, it } from "vitest";
import { safeExternalUrl } from "@/lib/safeExternalUrl";

describe("safeExternalUrl", () => {
  it("accepts https", () => {
    expect(safeExternalUrl("https://example.com/rel")).toBe("https://example.com/rel");
  });
  it("rejects javascript", () => {
    expect(safeExternalUrl("javascript:alert(1)")).toBeNull();
  });
  it("rejects empty", () => {
    expect(safeExternalUrl("")).toBeNull();
  });
});
