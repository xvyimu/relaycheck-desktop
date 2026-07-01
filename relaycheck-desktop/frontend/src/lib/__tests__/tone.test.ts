import { describe, it, expect } from "vitest";
import { statusTone, toneBadgeVariant, type Tone, type ToneBadgeVariant } from "../tone";

describe("statusTone", () => {
  it("returns 'good' for success-like values", () => {
    expect(statusTone("success")).toBe("good");
    expect(statusTone("ok")).toBe("good");
    expect(statusTone("healthy")).toBe("good");
    expect(statusTone("active")).toBe("good");
    expect(statusTone("valid")).toBe("good");
    expect(statusTone("enabled")).toBe("good");
  });

  it("returns 'bad' for failure-like values", () => {
    expect(statusTone("failed")).toBe("bad");
    expect(statusTone("error")).toBe("bad");
    expect(statusTone("danger")).toBe("bad");
    expect(statusTone("critical")).toBe("bad");
    expect(statusTone("invalid")).toBe("bad");
    expect(statusTone("expired")).toBe("bad");
    expect(statusTone("unreachable")).toBe("bad");
  });

  it("returns 'warn' for warning-like values", () => {
    expect(statusTone("warning")).toBe("warn");
    expect(statusTone("warn")).toBe("warn");
    expect(statusTone("missing")).toBe("warn");
    expect(statusTone("archived")).toBe("warn");
    expect(statusTone("unchecked")).toBe("warn");
  });

  it("returns 'warn' by default for 'unknown' (default unknown option)", () => {
    expect(statusTone("unknown")).toBe("warn");
  });

  it("respects custom unknown option", () => {
    expect(statusTone("unknown", { unknown: "neutral" })).toBe("neutral");
    expect(statusTone("unknown", { unknown: "bad" })).toBe("bad");
  });

  it("returns 'neutral' for unrecognized values", () => {
    expect(statusTone("pending")).toBe("neutral");
    expect(statusTone("running")).toBe("neutral");
  });

  it("is case-insensitive", () => {
    expect(statusTone("Success")).toBe("good");
    expect(statusTone("ERROR")).toBe("bad");
    expect(statusTone("Warning")).toBe("warn");
  });

  it("treats undefined as 'unknown'", () => {
    expect(statusTone(undefined)).toBe("warn");
  });

  it("treats empty string as 'unknown'", () => {
    expect(statusTone("")).toBe("warn");
  });
});

describe("toneBadgeVariant", () => {
  it("maps 'good' to 'success'", () => {
    expect(toneBadgeVariant("good")).toBe("success");
  });

  it("maps 'bad' to 'destructive'", () => {
    expect(toneBadgeVariant("bad")).toBe("destructive");
  });

  it("maps 'warn' to 'warning'", () => {
    expect(toneBadgeVariant("warn")).toBe("warning");
  });

  it("maps 'neutral' to 'secondary'", () => {
    expect(toneBadgeVariant("neutral")).toBe("secondary");
  });

  it("covers all Tone types exhaustively", () => {
    const tones: Tone[] = ["good", "bad", "warn", "neutral"];
    const variants: ToneBadgeVariant[] = tones.map(toneBadgeVariant);
    expect(variants).toEqual(["success", "destructive", "warning", "secondary"]);
  });
});
