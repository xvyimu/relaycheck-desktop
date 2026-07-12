import { describe, expect, it } from "vitest";
import { DialogShell } from "@/components/ui/dialog-shell";

describe("DialogShell contract", () => {
  it("exports a function component without chrome props", () => {
    expect(typeof DialogShell).toBe("function");
    expect(DialogShell.length).toBeGreaterThanOrEqual(1);
  });
});
