import { describe, expect, it } from "vitest";
import { useChannelActions } from "@/hooks/useChannelActions";
import { SettingsProxy } from "@/components/settings/SettingsProxy";
import { SettingsBackup } from "@/components/settings/SettingsBackup";
import { SettingsExportImport } from "@/components/settings/SettingsExportImport";

describe("S2 FE-7 panel contracts", () => {
  it("useChannelActions is a function accepting optional seed options", () => {
    expect(typeof useChannelActions).toBe("function");
    expect(useChannelActions.length).toBeLessThanOrEqual(1);
  });

  it("settings split modules export function components", () => {
    expect(typeof SettingsProxy).toBe("function");
    expect(typeof SettingsBackup).toBe("function");
    expect(typeof SettingsExportImport).toBe("function");
  });
});
